-include .env

MOCKERY_VERSION := v2.53.6
SWAG_VERSION := v1.16.6

.PHONY: run test test-integration vet lint fmt mocks swagger docker-up docker-down migrate-up migrate-down jwt

run:
	go run ./cmd/api

test:
	go test ./...

test-integration:
	INTEGRATION_TESTS=1 go test -tags=integration ./tests/integration -count=1

vet:
	go vet ./...

lint:
	GO111MODULE=on golangci-lint run ./...

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

mocks:
	GO111MODULE=on go run github.com/vektra/mockery/v2@$(MOCKERY_VERSION) --dir internal/application/company --name Repository --output internal/application/company/mocks --outpkg mocks --case underscore
	GO111MODULE=on go run github.com/vektra/mockery/v2@$(MOCKERY_VERSION) --dir internal/application/company --name EventProducer --output internal/application/company/mocks --outpkg mocks --case underscore
	GO111MODULE=on go run github.com/vektra/mockery/v2@$(MOCKERY_VERSION) --dir internal/application/company --name OutboxStore --output internal/application/company/mocks --outpkg mocks --case underscore
	GO111MODULE=on go run github.com/vektra/mockery/v2@$(MOCKERY_VERSION) --dir internal/application/company --name TransactionRunner --output internal/application/company/mocks --outpkg mocks --case underscore
	GO111MODULE=on go run github.com/vektra/mockery/v2@$(MOCKERY_VERSION) --dir internal/application/company --name Logger --output internal/application/company/mocks --outpkg mocks --case underscore
	GO111MODULE=on go run github.com/vektra/mockery/v2@$(MOCKERY_VERSION) --dir internal/infrastructure/outbox --name Repository --output internal/infrastructure/outbox/mocks --outpkg mocks --case underscore
	GO111MODULE=on go run github.com/vektra/mockery/v2@$(MOCKERY_VERSION) --dir internal/infrastructure/outbox --name TransactionRunner --output internal/infrastructure/outbox/mocks --outpkg mocks --case underscore

swagger:
	GO111MODULE=on go run github.com/swaggo/swag/cmd/swag@$(SWAG_VERSION) init -g cmd/api/main.go --parseInternal

docker-up:
	docker compose up --build

docker-down:
	docker compose down -v

migrate-up:
	migrate -path migrations -database "$$DATABASE_URL" up

migrate-down:
	migrate -path migrations -database "$$DATABASE_URL" down

jwt:
	go run ./cmd/token
