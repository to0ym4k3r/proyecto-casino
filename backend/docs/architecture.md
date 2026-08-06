# Casino Backend Architecture

## Overview

This backend follows **Hexagonal Architecture** (also known as Ports and Adapters), organizing code by business capability rather than technical layer.

## Layer Structure

```
cmd/server/main.go          → Entry point, wires everything together
internal/
  config/                    → Environment-based configuration
  domain/                    → Business models and DTOs (the core)
  repository/                → Data access interfaces (ports)
    postgres/                → PostgreSQL implementations (adapters)
  service/                   → Business logic (use cases)
  handler/                   → HTTP handlers (inbound adapters)
  middleware/                → Cross-cutting HTTP concerns
pkg/
  logger/                    → Structured logging utility
  response/                  → JSON response helpers
migrations/                  → SQL schema definitions
```

## Dependency Flow

```
handler → service → repository interfaces
                       ↑
              postgres (implements interfaces)
```

- **Domain** has zero dependencies — it defines the data structures
- **Repository interfaces** are defined in the repository package, implemented by postgres
- **Services** depend only on interfaces, not concrete implementations
- **Handlers** depend on services, parse HTTP, return JSON
- **Main** wires everything together with dependency injection

## Security Layers

1. **Input Validation** — Request parsing rejects malformed input early
2. **Rate Limiting** — Token-bucket per IP (auth: 5/min, general: 100/min)
3. **Authentication** — JWT with short-lived access tokens (15min) + refresh (7 days)
4. **Password Hashing** — bcrypt with cost factor 12
5. **SQL Injection Prevention** — All queries use parameterized statements ($1, $2...)
6. **Security Headers** — CSP, HSTS, X-Frame-Options, X-XSS-Protection
7. **CORS** — Configurable allowed origins

## Database

PostgreSQL 16 with pgx driver. Connection pooling via pgxpool with optimistic locking for balance updates.

## Running

```bash
# Local development
cp .env.example .env
docker-compose up -d postgres
make migrate
make run

# Docker (full stack)
docker-compose up -d --build
```
