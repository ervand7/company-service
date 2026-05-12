package company

import (
	"context"
	"time"

	companydomain "company-service/internal/domain/company"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

//go:generate go run github.com/vektra/mockery/v2@v2.53.6 --dir . --name Repository --output mocks --outpkg mocks --case underscore
type Repository interface {
	Create(ctx context.Context, company *companydomain.Company) error
	Update(ctx context.Context, company *companydomain.Company) error
	Delete(ctx context.Context, id uuid.UUID) error
	GetByID(ctx context.Context, id uuid.UUID) (*companydomain.Company, error)
	GetByName(ctx context.Context, name string) (*companydomain.Company, error)
}

//go:generate go run github.com/vektra/mockery/v2@v2.53.6 --dir . --name EventProducer --output mocks --outpkg mocks --case underscore
type EventProducer interface {
	Publish(ctx context.Context, event Event) error
	Close(ctx context.Context) error
}

//go:generate go run github.com/vektra/mockery/v2@v2.53.6 --dir . --name OutboxStore --output mocks --outpkg mocks --case underscore
type OutboxStore interface {
	Store(ctx context.Context, event Event) error
}

//go:generate go run github.com/vektra/mockery/v2@v2.53.6 --dir . --name TransactionRunner --output mocks --outpkg mocks --case underscore
type TransactionRunner interface {
	WithinTransaction(ctx context.Context, fn func(context.Context) error) error
}

//go:generate go run github.com/vektra/mockery/v2@v2.53.6 --dir . --name Logger --output mocks --outpkg mocks --case underscore
type Logger interface {
	Error() *zerolog.Event
}

type Event struct {
	ID         uuid.UUID
	Type       string
	OccurredAt time.Time
	CompanyID  uuid.UUID
	Company    *companydomain.Company
}

type OutboxEvent struct {
	ID       uuid.UUID
	Event    Event
	Attempts int
}

type CreateInput struct {
	ID                uuid.UUID
	Name              string
	Description       string
	AmountOfEmployees int
	Registered        bool
	Type              companydomain.CompanyType
}

type PatchInput struct {
	Name              *string
	Description       *string
	AmountOfEmployees *int
	Registered        *bool
	Type              *companydomain.CompanyType
}

const (
	EventCompanyCreated = "company.created"
	EventCompanyUpdated = "company.updated"
	EventCompanyDeleted = "company.deleted"
)
