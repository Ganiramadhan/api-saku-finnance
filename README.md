# Starter Go API

A minimal, production-ready Go API starter built with **Fiber**, **GORM (PostgreSQL)**, **JWT**, **Redis**, and **S3-compatible object storage** (MinIO out of the box). Designed as a clean foundation you can copy, rename, and grow into a real product without ripping out half the code.

## Highlights

- **Fiber v2** HTTP server with `cors`, `recover`, `requestid`, and structured request logging.
- **GORM + PostgreSQL** with connection pooling, soft deletes, prepared statements, and idempotent auto-migration.
- **JWT (HS256)** auth with role-based authorization (`user` / `admin`) and explicit valid-method allow-listing.
- **Validator** wrapper that returns human-friendly field errors via a centralized error handler.
- **Redis** cache with a no-op fallback so local dev never blocks on Redis being down.
- **S3 / MinIO storage** for user photo uploads — DB stores raw object keys, API responses include short-lived **presigned URLs**.
- **Swagger UI** auto-generated from handler annotations (`/swagger/index.html`).
- **Vertical-slice modules** (`handler / service / repository`) wired in [internal/app/app.go](internal/app/app.go).
- **Multi-stage distroless Docker image** running as non-root.
- **Makefile** + **docker-compose** for one-command local boots.
- **Unit tests** for the validator, JWT manager, HTTP helpers, middleware, and the user service (with in-memory fakes for repo + storage).
- Standard response envelope: `{ status, code, message, data, meta }`.

## Requirements

- Go **1.25+**
- PostgreSQL **14+**
- Redis **7+** (optional)
- MinIO or any S3-compatible store (local dev uses MinIO via Docker Compose)
- `make` (the `swag` and `air` CLIs are auto-installed by their make targets)

## Quick start

### Option A — full stack in Docker

```bash
cp envs/.env.example envs/.env
make up                 # postgres + redis + minio + minio-init + api
make logs               # follow logs
```

The API is reachable on `http://localhost:4000` and Swagger on `http://localhost:4000/swagger/index.html`.

### Option B — run the API locally, dependencies in Docker

```bash
cp envs/.env.example envs/.env
make up-deps            # postgres + redis + minio + bucket initializer
make swag               # generate Swagger docs (first run only)
make tidy
make run                # or `make dev` for hot reload via air
```

| URL | Purpose |
| --- | --- |
| `http://localhost:4000/health` | Liveness + dependency status |
| `http://localhost:4000/swagger/index.html` | Interactive API docs |
| `http://localhost:9001` | MinIO console (default `minioadmin` / `minioadmin`) |

## Project layout

```
starter-go/
├── cmd/api/                     # Process entrypoint + Swagger metadata
├── docker/Dockerfile            # Multi-stage distroless build
├── docker-compose.yml           # postgres + redis + minio + minio-init + api
├── docs/                        # Generated Swagger artefacts (git-ignored content)
├── envs/.env.example            # Sample configuration
├── internal/
│   ├── app/                     # Composition root (App struct, init*, Run, graceful shutdown)
│   ├── config/                  # Env-driven config loader (fail-fast)
│   ├── constants/               # User-facing message + error strings
│   ├── domain/                  # Entities + sentinel errors
│   ├── dto/                     # Request/response payloads + envelope
│   ├── middleware/              # Auth, role guard, central error handler
│   ├── modules/                 # Vertical slices: handler · service · repository
│   │   ├── auth/
│   │   └── user/
│   ├── platform/                # External integrations
│   │   ├── cache/               # Redis + Noop fallback
│   │   ├── database/            # Postgres connect + migration
│   │   └── storage/             # S3 / MinIO with presigned URLs
│   └── routes/                  # Route registration
├── pkg/                         # Small, reusable helpers
│   ├── httpx/                   # Bind / response writers / locals
│   ├── jwt/                     # Token sign + parse
│   └── validator/               # Validator wrapper with friendly errors
├── Makefile
└── README.md
```

## Endpoints

All endpoints are documented in Swagger. The summary below lists the surface area.

### Public

| Method | Path | Description |
| --- | --- | --- |
| `POST` | `/api/v1/auth/login` | Email + password login |
| `POST` | `/api/v1/auth/register` | Self-registration (role = `user`) |
| `GET`  | `/health` | Database + cache status |
| `GET`  | `/swagger/*` | Swagger UI |

### Authenticated user

| Method | Path | Description |
| --- | --- | --- |
| `POST`   | `/api/v1/auth/change-password` | Change my password |
| `GET`    | `/api/v1/users/me`             | Current profile |
| `PUT`    | `/api/v1/users/me`             | Update my profile (role is silently dropped) |
| `POST`   | `/api/v1/users/upload-photo`   | Upload an image to `temp/users/`, returns the object key |
| `DELETE` | `/api/v1/users/me/photo`       | Detach + delete my photo |

### Admin (`role=admin`)

| Method | Path | Description |
| --- | --- | --- |
| `GET`    | `/api/v1/admin/users`        | List users (page/limit/search) |
| `GET`    | `/api/v1/admin/users/{id}`   | Get user by ID |
| `POST`   | `/api/v1/admin/users`        | Create user |
| `PUT`    | `/api/v1/admin/users/{id}`   | Update user |
| `DELETE` | `/api/v1/admin/users/{id}`   | Soft-delete user (and remove photo) |

### Photo upload flow

The photo flow uses a **two-step "promote on save"** pattern so we never write the user record until the file is persisted, and we never leak orphan files when the DB transaction fails.

1. `POST /api/v1/users/upload-photo` — multipart `image` field (jpeg / png / webp, ≤ 5 MB). Returns:
   ```json
   {
     "data": {
       "image": "temp/users/avatar-1a2b3c4d.png",
       "preview_url": "https://minio.local/starter/temp/users/avatar-1a2b3c4d.png?X-Amz-...",
       "preview_expires_in": 604800
     }
   }
   ```
2. `PUT /api/v1/users/me` (or admin Create / Update) with `"photo": "temp/users/avatar-1a2b3c4d.png"`.
   The service moves the object to `users/<user-id>/avatar-1a2b3c4d.png`, persists the new key, deletes the previous photo (if any) from storage, **and rolls back the moved object if the DB write fails**.
3. Responses always include `photo` (raw key for storage) and `photo_url` (a 24-hour presigned GET URL the frontend can render directly).

## Response envelope

Every JSON response uses the same envelope so clients can rely on a single shape:

```json
{
  "status": "success",
  "code": 200,
  "message": "Successfully retrieved user",
  "data": { },
  "meta": {
    "page": 1, "limit": 10, "total": 42,
    "total_pages": 5, "has_next": true, "has_previous": false
  }
}
```

Validation failures return `422` with field-level details under `data.errors[]`.

## Configuration

All runtime configuration is driven by environment variables — see [envs/.env.example](envs/.env.example).

| Variable | Notes |
| --- | --- |
| `APP_PORT`, `APP_ENV`, `CORS_ORIGINS` | HTTP server basics |
| `DB_*` | Postgres host, credentials, pool tuning |
| `REDIS_*` | Optional — when unreachable the app falls back to a no-op cache |
| `JWT_SECRET`, `JWT_TTL_HOURS` | HS256 signing secret + token lifetime |
| `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` | Object-storage credentials |
| `AWS_DEFAULT_REGION` | e.g. `us-east-1` (required even for MinIO) |
| `AWS_ENDPOINT` | Leave empty for real AWS; set to `http://minio:9000` for MinIO |
| `AWS_BUCKET` | Bucket name; auto-created on boot if missing |
| `AWS_USE_PATH_STYLE_ENDPOINT` | `true` for MinIO, `false` for AWS |

> When running with `make up` (full Compose stack), the API container automatically overrides `DB_HOST`, `REDIS_HOST`, and `AWS_ENDPOINT` to the in-network service names — your `envs/.env` can keep the localhost values used by Option B.

## Adding a new module

Each domain lives in its own folder under `internal/modules/<name>/` with three files:

```
internal/modules/<name>/
  ├── repository.go   # GORM access — returns domain entities
  ├── service.go      # Business rules + DTO mapping
  └── handler.go      # Fiber handlers + Swagger annotations
```

Then wire it in [internal/app/app.go](internal/app/app.go) and register routes in [internal/routes/routes.go](internal/routes/routes.go). New entities should be appended to [internal/platform/database/migration.go](internal/platform/database/migration.go) so `AutoMigrate` picks them up.

## Testing

Unit tests cover the framework-agnostic logic — no Postgres, Redis, or S3 required.

```bash
make test         # go test -race -count=1 ./...
make cover        # writes coverage.html
```

| Package | What it covers |
| --- | --- |
| [internal/dto](internal/dto/response_test.go) | Pagination meta math (edge cases: empty, exact multiples, zero limit). |
| [internal/middleware](internal/middleware/middleware_test.go) | `AuthRequired` (missing header / wrong prefix / empty token / invalid / valid), `RequireAdmin`, and `ErrorHandler` mapping for every domain sentinel + Fiber + unknown errors. |
| [internal/modules/user](internal/modules/user/service_test.go) | `Service` happy paths plus the "promote on save" photo flow — including **rollback** of the moved S3 object when the DB write fails (the original bug we hardened against). Uses in-memory `Repository` + `Storage` fakes. |
| [pkg/httpx](pkg/httpx/httpx_test.go) | `ParseUUID`, `Bind` (valid / malformed JSON / validation error), and `UserID`/`Role` locals. |
| [pkg/jwt](pkg/jwt/jwt_test.go) | Round-trip generate→validate, wrong secret, expired token, malformed input, and **explicit rejection of `alg=none` tokens** (algorithm-confusion guard). |
| [pkg/validator](pkg/validator/validator_test.go) | Field-level errors keyed by JSON tag and the human-friendly message map. |

Add new tests next to the file under test using the `_test.go` suffix; the suite stays hermetic so `go test ./...` runs in seconds without external services.

## Make targets

Run `make help` for the full list with descriptions. Highlights:

| Target | Description |
| --- | --- |
| `make tidy` | `go mod tidy` |
| `make swag` | Generate Swagger docs into `docs/` (auto-installs `swag` CLI) |
| `make build` | Build the static binary into `bin/starter-go` (with `-ldflags` version) |
| `make run` | Run the API locally |
| `make dev` | Hot reload via [air](https://github.com/air-verse/air) (auto-installed) |
| `make test` / `make cover` | Run tests with `-race` (and HTML coverage) |
| `make vet` / `make fmt` / `make lint` | Static analysis |
| `make docker-build` / `make docker-run` | Build & run the production image |
| `make up` / `make up-deps` / `make down` / `make down-volumes` / `make logs` / `make ps` | Compose lifecycle |
| `make clean` | Remove build artefacts |

## Docker

The image is built in two stages:

1. **`golang:1.25-alpine`** with BuildKit cache mounts for module downloads and the build cache.
2. **`gcr.io/distroless/static-debian12:nonroot`** — no shell, no package manager, runs as `nonroot`. The final image only contains the static binary plus `envs/`.

```bash
make docker-build       # build the image
make up                 # bring up postgres + redis + minio + bucket initializer + API
```

## Security notes

- Never commit a real `envs/.env` — it is git- and docker-ignored.
- Rotate `JWT_SECRET` per environment; never reuse the example value in production.
- JWT validation is locked to `HS256` via `WithValidMethods` to prevent algorithm confusion attacks.
- MinIO defaults (`minioadmin` / `minioadmin`) are for local dev only — change them before exposing the service.
- The `UpdateMe` handler intentionally drops `role` from the request body to prevent self-elevation.
- Uploads are bound by `MaxUploadSize` (5 MB) and a content-type allow-list (`jpeg`/`png`/`webp`); tighten further as needed.
- Auth middleware requires the `Bearer ` prefix and rejects empty tokens.

## License

MIT — use it, fork it, ship it.
