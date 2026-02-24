# Contributing

Issues and pull requests are welcome.

## Getting started

Requires Go 1.21+.

Compile:

```bash
git clone https://github.com/neoclaw-ai/neoclaw.git
cd neoclaw
go build -o bin/claw ./cmd/claw
```

Run the tests:

```bash
go test ./...
```

## Before submitting a PR

- Run `gofmt` on any Go files you've changed.
- Make sure `go test ./...` passes.
- Keep changes focused — one concern per PR.

## Reporting issues

Please include:
- Your OS and Go version (`go version`)
- Steps to reproduce
- What you expected vs. what happened

## Project philosophy

NeoClaw is intentionally lean. Before adding something, consider whether it fits the 80/20 rule — does it solve a real need for most users, or is it a niche edge case? Simple, readable code is preferred over clever code. See the architecture notes in the README for more context.

We follow the Unix philosophy: Small, sharp tools.
