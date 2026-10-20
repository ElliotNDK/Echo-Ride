# Go-EchoRide

Go-EchoRide is a Go-based microservices backend for a ride-hailing platform.  
It combines REST APIs, WebSocket communication, gRPC, and event-driven workflows to handle authentication, user profiles (roles and account status), ride lifecycle management, real-time location updates, and driver-rider matching.

## Project Scope

- Model core ride-hailing backend flows in a distributed architecture.
- Demonstrate service-to-service communication using HTTP, Kafka, gRPC, and WebSocket.
- Provide a practical local development environment with Docker Compose and per-service workflows.

## Tech Stack

- Go `1.25.7`
- Echo v5 (HTTP API layer)
- PostgreSQL (transactional data)
- Redis (cache and short-lived state)
- Kafka (event streaming)
- Cassandra (location persistence)
- APISIX + APISIX Dashboard (API Gateway)
- Jaeger + OpenTelemetry (distributed tracing)
- Debezium (CDC from PostgreSQL to Kafka)
- OSRM (routing and distance calculations)

## Architecture Overview

### Core Services

- `services/auth-service`  
  Handles credentials (identities, password hashes), refresh-token sessions, JWT issuance, and Redis-backed access-token revocation. At login/refresh it calls `user-service` over gRPC for role and account status before embedding claims in the JWT.

- `services/user-service`  
  Owns user profile data and is the source of truth for `role` and `account_status`. Exposes REST (`/api/v1/users/me`, admin role/status updates), gRPC (`GetUserAuthInfo` for `auth-service`), and consumes Kafka events such as `IDENTITY_CREATED` to upsert rows in `t_users`.

- `services/ride-service`  
  Manages ride domain logic and consumes/publishes ride events.

- `services/location-service`  
  Ingests real-time location updates via WebSocket, exposes gRPC endpoints, and integrates with Redis, Cassandra, and OSRM.

- `services/matching-service`  
  Processes dispatch and matching flow using Kafka + Redis, with gRPC integration to `location-service`.

- `services/payment-service`  
  Minimal payment service scaffold for future expansion.

- `pkg-library`  
  Shared modules (for example: tracing, gRPC contracts, common helpers).

### Local Infrastructure

Defined in `infra/development/docker/docker-compose.dev.yml`:

- Gateway: `etcd`, `apisix`, `apisix-dashboard`
- Data and messaging: `postgres`, `redis`, `kafka`, `debezium`, `cassandra`
- Platform services: `jaeger`, `osrm`

## Repository Structure

```text
.
├── go.mod
├── go.work
├── Makefile
├── pkg-library/
├── services/
│   ├── auth-service/
│   ├── user-service/
│   ├── ride-service/
│   ├── location-service/
│   ├── matching-service/
│   └── payment-service/
├── infra/
│   └── development/
│       └── docker/
└── scripts/
    └── apisix-script
```

## Prerequisites

- Go `1.25.x`
- Docker and Docker Compose
- Make
- Recommended development tools:
  - `air` for hot reload
  - `goose` for database migrations
  - `jq` for APISIX bootstrap script support

## Quick Start

### 1. Clone the repository

```bash
git clone <repo-url>
cd Go-EchoRide
```

### 2. Start local infrastructure

```bash
make docker-up
```

View logs:

```bash
make docker-logs
```

Stop infrastructure:

```bash
make docker-down
```

### 3. Run services

From the repository root:

```bash
make run service=auth-service
make run service=user-service
make run service=ride-service
make run service=location-service
make run service=matching-service
make run service=payment-service
```

Hot reload (example):

```bash
make dev service=ride-service
```

Build all services:

```bash
make build-all
```

## Database Migrations

Migration targets are currently available for:

- `services/auth-service`
- `services/user-service`
- `services/ride-service`

Apply migrations:

```bash
make migrate-up service=auth-service
make migrate-up service=user-service
make migrate-up service=ride-service
```

Create a new migration:

```bash
make migrate-create service=ride-service name=add_new_table
```

## API Gateway and JWT Setup

The script `scripts/apisix-script` bootstraps:

- Auth route: `/api/v1/auth/*` (upstream `auth-service`)
- JWT consumer in APISIX (must match the `key` claim in tokens issued by `auth-service`)
- Protected ride route: `/api/v1/rides*` with JWT + header injection for `X-User-Id` / `X-User-Role`
- WebSocket proxy route: `/ws*` (upstream `location-service`)

For routes that also cover `user-service`, split public auth (`register` / `login` / `refresh`) vs JWT-protected paths, and the same header injection pattern, use `infra/development/apisix/apisix_admin_setup.sh` (see `infra/development/apisix/routes/` and `inject_user_headers.lua`; requires `jq` and `envsubst`).

Usage:

```bash
export APISIX_JWT_SECRET="<must match auth-service jwt.secret>"
bash scripts/apisix-script
```

## Service Ports

### Application services (`config.dev.yml`)

- Auth Service: `8114`
- User Service HTTP: `8115`
- User Service gRPC: `9115`
- Ride Service: `8111`
- Location Service HTTP: `8112`
- Location Service gRPC: `50052`
- Matching Service HTTP (health): `8113`
- Payment Service: `8083`

### Infrastructure services (`docker-compose.dev.yml`)

- APISIX Gateway: `9080`
- APISIX Admin API: `9180`
- APISIX Dashboard: `9000`
- PostgreSQL: `5441` (container `5432`)
- Redis: `6379`
- Kafka (external): `9092`
- Debezium: `8083`
- Jaeger UI: `16686`
- OSRM: `5000`
- Cassandra: `9042`

## Observability

- Shared tracing is initialized via `pkg-library/tracing`.
- Key services (including `auth-service`, `user-service`, `ride-service`, and `location-service`) export traces to Jaeger.
- Jaeger UI: `http://localhost:16686`

## Health and Verification

User service health endpoint:

```bash
curl http://localhost:8115/health
```

Matching service health endpoint:

```bash
curl http://localhost:8113/health
```

List APISIX routes:

```bash
curl -H "X-API-KEY: <apisix-admin-key>" http://localhost:9180/apisix/admin/routes
```

## Configuration Notes

- Development configuration files are located at `services/*/config/config.dev.yml`.
- If services run on host while dependencies run in Docker, verify host/port values carefully.
- Keep JWT secrets aligned between `auth-service` and APISIX (`APISIX_JWT_SECRET`).
- `auth-service` must reach `user-service` gRPC at `AUTH_USER_SERVICE_GRPC_ADDR` (default `localhost:9115`) so login and refresh can load role and status.

## Roadmap Suggestions

- Add integration tests for the end-to-end flow: ride -> matching -> location.
- Standardize migration and bootstrap tooling across all services.
- Expand `payment-service` from scaffold to full payment workflow.
- Add CI for lint, test, and build validation.

<!-- ## License

No license file is currently included.  
If this repository is intended for public distribution, add a `LICENSE` file. -->
