# Helix

[![CI](https://github.com/enokdev/helix/actions/workflows/ci.yml/badge.svg)](https://github.com/enokdev/helix/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/enokdev/helix)](https://goreportcard.com/report/github.com/enokdev/helix)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

Helix is a Go backend framework inspired by Spring Boot, built on top of [Fiber](https://gofiber.io/).

It provides idiomatic Go equivalents of Spring Boot concepts: DI/IoC container, auto-configuration, a generic repository pattern, observability endpoints, and a CLI scaffolding tool.

## Quick Start

```bash
# Clone the repository
git clone https://github.com/enokdev/helix.git
cd helix

# Install dependencies
go mod tidy

# Build
go build ./...

# Run tests
go test ./...
```

## Requirements

- Go 1.21+

## Project Structure

```
helix/
├── core/          # DI container, lifecycle management
├── config/        # YAML/JSON/TOML/ENV configuration loader
├── web/           # Fiber HTTP integration, routing, middleware
├── data/          # Repository pattern, ORM adapters
├── starter/       # Auto-configuration modules
├── observability/ # Prometheus metrics, slog, OpenTelemetry
├── security/      # JWT auth, RBAC middleware
├── scheduler/     # Declarative task scheduling
├── cli/           # Project/module generator
└── examples/      # Example applications
```

## License

MIT — see [LICENSE](LICENSE).
