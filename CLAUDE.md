# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Module

`github.com/jiaguoliang/huashunxinan` — Go library, built with Go 1.12.

## Commands

```bash
# Build
go build ./...

# Test
go test ./...

# Test a single package
go test ./some/package/...

# Run a single test
go test -run TestFunctionName ./...

# Test with verbose output
go test -v ./...

# Lint (if golint or staticcheck is available)
golint ./...
# or
staticcheck ./...

# Format
gofmt -w .
```

## Architecture

This is a Go library. The root package is `huashunxinan` (external test package: `huashunxinan_test`).

- Library code lives in `.go` files at the root or in subdirectory packages.
- Test files use the `_test.go` suffix. External black-box tests use the `huashunxinan_test` package; white-box tests use `huashunxinan`.
- Dependencies are managed via `go.mod` / `go.sum` — run `go get` to add, `go mod tidy` to clean up.
