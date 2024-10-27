package uvgo

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Runner is a Python script runner using the UV tool
type Runner struct {
	pythonVersion string
	extraFlags    []string
	timeout       time.Duration
	env           []string
	workDir       string
	dependencies  []string
	scriptArgs    []string
}

// Option represents a configuration option for the Runner
type Option func(*Runner)

// New creates a new UV runner with the provided options
func New(options ...Option) (*Runner, error) {
	_, err := exec.LookPath("uv")
	if err != nil {
		return nil, fmt.Errorf("uv not found in PATH: %w", err)
	}

	r := &Runner{timeout: 30 * time.Second}
	for _, opt := range options {
		opt(r)
	}
	return r, nil
}

// WithPython sets the python version to use
func WithPython(version string) Option {
	return func(r *Runner) { r.pythonVersion = version }
}

// WithExtraFlags sets the extra flags to pass to the UV command
func WithExtraFlags(flags ...string) Option {
	return func(r *Runner) { r.extraFlags = flags }
}

// WithTimeout sets the execution timeout
func WithTimeout(timeout time.Duration) Option {
	return func(r *Runner) { r.timeout = timeout }
}

// WithEnv sets additional environment variables
func WithEnv(env ...string) Option {
	return func(r *Runner) { r.env = env }
}

// WithWorkDir sets the working directory
func WithWorkDir(workDir string) Option {
	return func(r *Runner) { r.workDir = workDir }
}

// WithDependencies sets the Python dependencies to install
func WithDependencies(deps ...string) Option {
	return func(r *Runner) { r.dependencies = deps }
}

// WithScriptArgs sets the default arguments to pass to the Python script
func WithScriptArgs(args ...string) Option {
	return func(r *Runner) { r.scriptArgs = args }
}

// Result represents the output of a script execution
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Duration time.Duration
}

// Run executes a Python script from a file with optional arguments
func (r *Runner) Run(ctx context.Context, scriptPath string, args ...string) (*Result, error) {
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("script file does not exist: %w", err)
	}
	return r.execute(ctx, scriptPath, "", args)
}

// RunFromString executes a Python script from a string with optional arguments
func (r *Runner) RunFromString(ctx context.Context, script string, args ...string) (*Result, error) {
	if script == "" {
		return nil, fmt.Errorf("empty script provided")
	}
	return r.execute(ctx, "-", script, args)
}

func (r *Runner) execute(ctx context.Context, scriptPath, scriptContent string, args []string) (*Result, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	uvArgs := []string{"run"}

	if r.pythonVersion != "" {
		uvArgs = append(uvArgs, "--python", r.pythonVersion)
	}

	for _, dep := range r.dependencies {
		uvArgs = append(uvArgs, "--with", dep)
	}

	uvArgs = append(uvArgs, r.extraFlags...)
	uvArgs = append(uvArgs, scriptPath)

	var scriptArgs []string
	if len(args) > 0 {
		scriptArgs = args
	} else if len(r.scriptArgs) > 0 {
		scriptArgs = r.scriptArgs
	}

	if len(scriptArgs) > 0 {
		uvArgs = append(uvArgs, scriptArgs...)
	}

	cmd := exec.CommandContext(ctx, "uv", uvArgs...)

	if r.workDir != "" {
		cmd.Dir = r.workDir
	}

	if len(r.env) > 0 {
		cmd.Env = append(os.Environ(), r.env...)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if scriptPath == "-" {
		cmd.Stdin = strings.NewReader(scriptContent)
	}

	startTime := time.Now()
	err := cmd.Run()
	endTime := time.Now()

	result := &Result{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: cmd.ProcessState.ExitCode(),
		Duration: endTime.Sub(startTime),
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return result, fmt.Errorf("script execution timed out after %v: %w", r.timeout, err)
		}
		if exitError, ok := err.(*exec.ExitError); ok {
			if result.Stderr != "" {
				return result, fmt.Errorf("script execution failed: %s", result.Stderr)
			}
			return result, fmt.Errorf("script execution failed with exit code %d: %w", exitError.ExitCode(), err)
		}
		return result, fmt.Errorf("script execution failed: %w", err)
	}
	return result, nil
}
