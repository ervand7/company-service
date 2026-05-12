# Company Service

Production-ready Go microservice for managing companies. It exposes a small REST API backed by PostgreSQL, uses JWT authentication for mutating operations, and includes Kafka event publishing with a transactional outbox for reliable company events.

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
| `EVENT_PRODUCER` | Event producer implementation: `log` or `kafka` | `log` |
| `KAFKA_BROKERS` | Comma-separated Kafka broker addresses | `localhost:9092` |
| `KAFKA_TOPIC` | Kafka topic for company events | `company.events` |
| `OUTBOX_POLL_INTERVAL` | How often the outbox publisher polls pending events | `2s` |
| `OUTBOX_BATCH_SIZE` | Max outbox events processed per poll | `10` |
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

Both endpoints should return `{"status":"ok"}` when the application and database are ready.

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
    "id": "00000000-0000-0000-0000-000000000001",
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

Events include an event id, event type, occurrence time, company id, and useful company data. Mutating operations write the company change and an outbox event in the same database transaction. A background outbox publisher then publishes pending events using the configured producer and marks them as published.

Set `EVENT_PRODUCER=kafka` to publish JSON events to Kafka using `KAFKA_BROKERS` and `KAFKA_TOPIC`; set `EVENT_PRODUCER=log` to log events as structured JSON. `docker compose up --build` starts Kafka and uses the Kafka producer by default.

If publishing fails, the API request still succeeds once the database transaction commits. The outbox row remains pending, records the failure, and is retried by the publisher.

### Kafka Smoke Test

When the stack is running with Docker Compose, open a Kafka consumer in one terminal:

```sh
docker compose exec kafka /opt/kafka/bin/kafka-console-consumer.sh \
  --bootstrap-server localhost:9092 \
  --topic company.events \
  --from-beginning
```

If this prints `UNKNOWN_TOPIC_OR_PARTITION` before any company is created, that is expected. The topic is auto-created on the first publish. You can also create it manually:

```sh
docker compose exec kafka /opt/kafka/bin/kafka-topics.sh \
  --bootstrap-server localhost:9092 \
  --create \
  --if-not-exists \
  --topic company.events \
  --partitions 1 \
  --replication-factor 1
```

In another terminal, generate a JWT and create a company:

```sh
TOKEN=$(JWT_SECRET=local-development-secret-change-me go run ./cmd/token)

curl -X POST http://localhost:8080/companies \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "id": "00000000-0000-0000-0000-000000000001",
    "name": "Acme",
    "description": "Example company",
    "amount_of_employees": 10,
    "registered": true,
    "type": "Corporations"
  }'
```

The consumer should print a JSON event such as `company.created`. The same flow applies to patch and delete operations with `company.updated` and `company.deleted`.

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
- Company IDs are supplied by the create request as UUIDs and are validated before persistence.
- The transactional outbox avoids losing events when Kafka is temporarily unavailable, but the publisher is still an in-process worker. A larger production deployment may move publishing to a separate worker service.
- The Kafka producer is configured through `EVENT_PRODUCER`, `KAFKA_BROKERS`, and `KAFKA_TOPIC`; the log producer remains available for lightweight local runs.
- Description is stored as an empty string when omitted, which keeps JSON and SQL simple while still representing an optional field at the API level.
- Name uniqueness is checked in the application layer and enforced with a database unique constraint. The DB constraint remains the source of truth under concurrency.
