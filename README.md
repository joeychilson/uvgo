# uvgo

A library for running the uv tool in Go.

## Installation

```bash
go get -u github.com/joeychilson/uvgo
```

## Usage

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"

    "github.com/joeychilson/uvgo"
)

func main() {
    ctx := context.Background()

    uv, err := uvgo.New()
    if err != nil {
        log.Fatalf("error creating runner: %v", err)
    }

    result, err := uv.RunFromString(ctx, `
# /// script
# dependencies = [
#   "requests<3",
# ]
# ///
import json
import requests

resp = requests.get("https://jsonplaceholder.typicode.com/todos/1")
data = resp.json()

print(json.dumps(data, indent=2))
    `)
    if err != nil {
        log.Fatalf("error running script: %v", err)
    }

    fmt.Printf("Duration: %v\n", result.Duration)

    var response Response
    err = json.Unmarshal([]byte(result.Stdout), &response)
    if err != nil {
        log.Fatalf("error unmarshalling response: %v", err)
    }

    fmt.Printf("Response: %+v\n", response)
}

type Response struct {
    UserID    int    `json:"userId"`
    ID        int    `json:"id"`
    Title     string `json:"title"`
    Completed bool   `json:"completed"`
}
