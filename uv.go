package uvgo

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
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
	Stdout     string
	Stderr     string
	SystemTime time.Duration
	UserTime   time.Duration
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

	err := cmd.Run()

	result := &Result{
		Stdout:     stdout.String(),
		Stderr:     stderr.String(),
		SystemTime: cmd.ProcessState.SystemTime(),
		UserTime:   cmd.ProcessState.UserTime(),
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

// StructuredResult adds typed data to the base Result
type StructuredResult[T any] struct {
	*Result
	Data T
}

// StructuredOutput runs a script and parses its output into the specified type
func StructuredOutput[T any](ctx context.Context, r *Runner, scriptPath string, args ...string) (*StructuredResult[T], error) {
	scriptContent, err := os.ReadFile(scriptPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read script file: %w", err)
	}

	if err := validateJSONPrint(string(scriptContent)); err != nil {
		return nil, fmt.Errorf("invalid script format: %w", err)
	}

	result, err := r.Run(ctx, scriptPath, args...)
	if err != nil {
		return &StructuredResult[T]{Result: result}, err
	}

	var output T
	if err := json.Unmarshal([]byte(result.Stdout), &output); err != nil {
		return &StructuredResult[T]{Result: result}, fmt.Errorf("failed to unmarshal script output: %w", err)
	}

	return &StructuredResult[T]{
		Result: result,
		Data:   output,
	}, nil
}

// StructuredOutputFromString runs a script from a string and parses its output into the specified type
func StructuredOutputFromString[T any](ctx context.Context, r *Runner, script string, args ...string) (*StructuredResult[T], error) {
	if err := validateJSONPrint(script); err != nil {
		return nil, fmt.Errorf("invalid script format: %w", err)
	}

	result, err := r.RunFromString(ctx, script, args...)
	if err != nil {
		return &StructuredResult[T]{Result: result}, err
	}

	var output T
	if err := json.Unmarshal([]byte(result.Stdout), &output); err != nil {
		return &StructuredResult[T]{Result: result}, fmt.Errorf("failed to unmarshal script output: %w", err)
	}

	return &StructuredResult[T]{
		Result: result,
		Data:   output,
	}, nil
}

func validateJSONPrint(script string) error {
	if strings.TrimSpace(script) == "" {
		return fmt.Errorf("empty script provided")
	}

	scanner := bufio.NewScanner(strings.NewReader(script))
	var lastNonEmptyLine string

	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	for i := len(lines) - 1; i >= 0; i-- {
		if line := strings.TrimSpace(lines[i]); line != "" {
			lastNonEmptyLine = line
			break
		}
	}

	if !strings.Contains(lastNonEmptyLine, "print(json.dumps") {
		return fmt.Errorf("script must end with print(json.dumps(...)) for structured output")
	}
	return nil
}
