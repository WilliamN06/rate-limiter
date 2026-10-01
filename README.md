# Rate Limiter as a Service

A self-hostable, single-binary rate limiting service for indie developers and small teams.

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

##  Features

-  **Single binary** — No dependencies, just download and run
- **Embedded storage** — SQLite with in-memory caching
- **Multiple algorithms** — Fixed window, sliding window, token bucket
-  **Simple config** — YAML configuration with environment variable overrides
-  **Observability** — Prometheus metrics and structured logging
- **TypeScript client** — First-class support for Node.js/Deno/Bun
-  **Docker ready** — Official Docker image and Docker Compose
-  **Fast** — <100µs per request at 500+ RPS

##  Quick Start

### 1. Download the binary

```bash
# Linux
curl -L https://github.com/WilliamN06/rate-limiter/releases/latest/download/rate-limiter-linux-amd64 -o rate-limiter
chmod +x rate-limiter

# macOS
curl -L https://github.com/WilliamN06/rate-limiter/releases/latest/download/rate-limiter-darwin-amd64 -o rate-limiter
chmod +x rate-limiter