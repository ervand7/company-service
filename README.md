# Company Service

Production-ready Go microservice for managing companies. It exposes a small REST API backed by PostgreSQL, uses JWT authentication for mutating operations, and publishes company events through an application-level event producer interface.

## Requirements

- Go 1.23+
- Docker and Docker Compose
- Optional local tools: `migrate`, `golangci-lint`

## Configuration

Copy `.env.example` to `.env` for local development.

| Variable | Description | Default |
| --- | --- | --- |
| `HTTP_ADDR` | HTTP listen address | `:8080` |
| `DATABASE_URL` | PostgreSQL connection string | required |
| `JWT_SECRET` | HMAC secret for JWT validation | required |
| `EVENT_PRODUCER` | Event producer implementation | `log` |
| `DB_MAX_CONNS` | Max DB pool connections | `10` |
| `DB_MIN_CONNS` | Min DB pool connections | `1` |

Never commit real JWT secrets.

## Run Locally

Start Postgres:

```sh
docker compose up postgres migrate
```

Run the service locally:

```sh
DATABASE_URL='your_database_url' \
JWT_SECRET='your_secret' \
go run ./cmd/api
```

Run everything with Docker Compose:

```sh
docker compose up --build
```

Health checks:

```sh
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
```

Swagger UI is available when the service is running:

```sh
open http://localhost:8080/swagger/index.html
```

The generated OpenAPI document is served at `http://localhost:8080/swagger/doc.json`. Regenerate the checked-in Swagger files after changing API annotations:

```sh
go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -g cmd/api/main.go --parseInternal
```

## JWT for Testing

Generate a token using the same `JWT_SECRET` as the service:

```sh
JWT_SECRET=local-development-secret-change-me go run ./cmd/token
```

Use it on protected endpoints:

```sh
TOKEN=$(JWT_SECRET=local-development-secret-change-me go run ./cmd/token)
```

## API Examples

Create company:

```sh
curl -X POST http://localhost:8080/companies \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Acme",
    "description": "Example company",
    "amount_of_employees": 10,
    "registered": true,
    "type": "Corporations"
  }'
```

Get company:

```sh
curl http://localhost:8080/companies/<company-id>
```

Patch company:

```sh
curl -X PATCH http://localhost:8080/companies/<company-id> \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"registered": false, "amount_of_employees": 12}'
```

Delete company:

```sh
curl -X DELETE http://localhost:8080/companies/<company-id> \
  -H "Authorization: Bearer $TOKEN"
```

## Events

The service publishes these events after successful database mutations:

- `company.created`
- `company.updated`
- `company.deleted`

Events include an event id, event type, occurrence time, company id, and useful company data. The default producer logs events as structured JSON. Event publish failures are logged and returned as operation errors; this means a DB mutation can succeed while the API returns an error if the configured event producer fails. A real Kafka producer can be added behind `EventProducer` without changing domain or use case code.

## Tests and Checks

Run unit tests:

```sh
go test ./...
```

Run integration tests with testcontainers:

```sh
INTEGRATION_TESTS=1 go test -tags=integration ./tests/integration -count=1
```

Run vet and linter:

```sh
go vet ./...
golangci-lint run ./...
```

Format code:

```sh
gofmt -w $(find . -name '*.go' -not -path './vendor/*')
```

## Trade-offs and Assumptions

- `GET /companies/{id}` is public; POST, PATCH, and DELETE require a valid JWT.
- The local event producer logs events. Kafka support is intentionally behind an interface and can be added without changing business logic.
- Description is stored as an empty string when omitted, which keeps JSON and SQL simple while still representing an optional field at the API level.
- Name uniqueness is checked in the application layer and enforced with a database unique constraint. The DB constraint remains the source of truth under concurrency.
