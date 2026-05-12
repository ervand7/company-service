package company_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	appcompany "company-service/internal/application/company"
	"company-service/internal/application/company/mocks"
	companydomain "company-service/internal/domain/company"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
)

var testLogger = zerolog.Nop()

func TestCreateCompanySuccess(t *testing.T) {
	ctx := context.Background()
	repo := mocks.NewRepository(t)
	events := mocks.NewEventProducer(t)
	service := appcompany.NewService(repo, events, &testLogger)

	input := appcompany.CreateInput{
		Name:              "  Acme  ",
		Description:       "  description  ",
		AmountOfEmployees: 10,
		Registered:        true,
		Type:              companydomain.TypeCorporations,
	}

	repo.On("GetByName", ctx, "Acme").Return((*companydomain.Company)(nil), companydomain.ErrNotFound).Once()
	repo.On("Create", ctx, mock.MatchedBy(func(c *companydomain.Company) bool {
		return c.ID != uuid.Nil &&
			c.Name == "Acme" &&
			c.Description == "description" &&
			c.AmountOfEmployees == 10 &&
			c.Registered &&
			c.Type == companydomain.TypeCorporations &&
			!c.CreatedAt.IsZero() &&
			!c.UpdatedAt.IsZero()
	})).Return(nil).Once()
	events.On("Publish", ctx, mock.MatchedBy(func(event appcompany.Event) bool {
		return event.ID != uuid.Nil &&
			event.Type == appcompany.EventCompanyCreated &&
			event.CompanyID != uuid.Nil &&
			event.Company != nil &&
			event.CompanyID == event.Company.ID &&
			event.Company.Name == "Acme" &&
			!event.OccurredAt.IsZero()
	})).Return(nil).Once()

	created, err := service.CreateCompany(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if created.Name != "Acme" {
		t.Fatalf("expected trimmed company name, got %q", created.Name)
	}
}

func TestCreateCompanyReturnsValidationError(t *testing.T) {
	ctx := context.Background()
	repo := mocks.NewRepository(t)
	events := mocks.NewEventProducer(t)
	service := appcompany.NewService(repo, events, &testLogger)

	_, err := service.CreateCompany(ctx, appcompany.CreateInput{
		Name:              "",
		AmountOfEmployees: -1,
		Type:              "invalid",
	})
	if err == nil {
		t.Fatal("expected validation error")
	}

	var validationErr companydomain.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	repo.AssertNotCalled(t, "GetByName", mock.Anything, mock.Anything)
	repo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	events.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything)
}

func TestCreateCompanyReturnsDuplicateName(t *testing.T) {
	ctx := context.Background()
	repo := mocks.NewRepository(t)
	events := mocks.NewEventProducer(t)
	service := appcompany.NewService(repo, events, &testLogger)
	existing, newErr := companydomain.New("Acme", "description", 10, true, companydomain.TypeCorporations)
	if newErr != nil {
		t.Fatal(newErr)
	}

	repo.On("GetByName", ctx, "Acme").Return(existing, nil).Once()

	_, err := service.CreateCompany(ctx, validCreateInput("Acme"))
	if !errors.Is(err, companydomain.ErrDuplicateName) {
		t.Fatalf("expected ErrDuplicateName, got %v", err)
	}
	repo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	events.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything)
}

func TestCreateCompanyReturnsLookupError(t *testing.T) {
	ctx := context.Background()
	repo := mocks.NewRepository(t)
	events := mocks.NewEventProducer(t)
	service := appcompany.NewService(repo, events, &testLogger)
	lookupErr := errors.New("lookup failed")

	repo.On("GetByName", ctx, "Acme").Return((*companydomain.Company)(nil), lookupErr).Once()

	_, err := service.CreateCompany(ctx, validCreateInput("Acme"))
	if !errors.Is(err, lookupErr) {
		t.Fatalf("expected lookup error, got %v", err)
	}
	repo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	events.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything)
}

func TestCreateCompanyReturnsCreateError(t *testing.T) {
	ctx := context.Background()
	repo := mocks.NewRepository(t)
	events := mocks.NewEventProducer(t)
	service := appcompany.NewService(repo, events, &testLogger)
	createErr := errors.New("create failed")

	repo.On("GetByName", ctx, "Acme").Return((*companydomain.Company)(nil), companydomain.ErrNotFound).Once()
	repo.On("Create", ctx, mock.AnythingOfType("*company.Company")).Return(createErr).Once()

	_, err := service.CreateCompany(ctx, validCreateInput("Acme"))
	if !errors.Is(err, createErr) {
		t.Fatalf("expected create error, got %v", err)
	}
	events.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything)
}

func TestCreateCompanyReturnsPublishError(t *testing.T) {
	ctx := context.Background()
	repo := mocks.NewRepository(t)
	events := mocks.NewEventProducer(t)
	service := appcompany.NewService(repo, events, &testLogger)
	publishErr := errors.New("publish failed")

	repo.On("GetByName", ctx, "Acme").Return((*companydomain.Company)(nil), companydomain.ErrNotFound).Once()
	repo.On("Create", ctx, mock.AnythingOfType("*company.Company")).Return(nil).Once()
	events.On("Publish", ctx, mock.MatchedBy(func(event appcompany.Event) bool {
		return event.Type == appcompany.EventCompanyCreated
	})).Return(publishErr).Once()

	_, err := service.CreateCompany(ctx, validCreateInput("Acme"))
	if err == nil {
		t.Fatal("expected publish error")
	}
	if !errors.Is(err, publishErr) {
		t.Fatalf("expected wrapped publish error, got %v", err)
	}
	if !strings.Contains(err.Error(), "publish company event") {
		t.Fatalf("expected publish error context, got %v", err)
	}
}

func TestPatchCompanySuccessWithoutNameLookup(t *testing.T) {
	ctx := context.Background()
	id := uuid.New()
	repo := mocks.NewRepository(t)
	events := mocks.NewEventProducer(t)
	service := appcompany.NewService(repo, events, &testLogger)
	existing, newErr := companydomain.New("Acme", "description", 10, true, companydomain.TypeCorporations)
	if newErr != nil {
		t.Fatal(newErr)
	}
	existing.ID = id
	description := "updated"
	employees := 20
	registered := false
	companyType := companydomain.TypeNonProfit

	repo.On("GetByID", ctx, id).Return(existing, nil).Once()
	repo.On("Update", ctx, mock.MatchedBy(func(c *companydomain.Company) bool {
		return c.ID == id &&
			c.Name == "Acme" &&
			c.Description == description &&
			c.AmountOfEmployees == employees &&
			!c.Registered &&
			c.Type == companyType
	})).Return(nil).Once()
	events.On("Publish", ctx, mock.MatchedBy(func(event appcompany.Event) bool {
		return event.Type == appcompany.EventCompanyUpdated &&
			event.CompanyID == id &&
			event.Company == existing
	})).Return(nil).Once()

	updated, err := service.PatchCompany(ctx, id, appcompany.PatchInput{
		Description:       &description,
		AmountOfEmployees: &employees,
		Registered:        &registered,
		Type:              &companyType,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated != existing {
		t.Fatal("expected service to return updated existing company")
	}
	repo.AssertNotCalled(t, "GetByName", mock.Anything, mock.Anything)
}

func TestPatchCompanySuccessWithSameName(t *testing.T) {
	ctx := context.Background()
	id := uuid.New()
	repo := mocks.NewRepository(t)
	events := mocks.NewEventProducer(t)
	service := appcompany.NewService(repo, events, &testLogger)
	existing, newErr := companydomain.New("Acme", "description", 10, true, companydomain.TypeCorporations)
	if newErr != nil {
		t.Fatal(newErr)
	}
	existing.ID = id
	name := "  Acme  "

	repo.On("GetByID", ctx, id).Return(existing, nil).Once()
	repo.On("GetByName", ctx, "Acme").Return(existing, nil).Once()
	repo.On("Update", ctx, mock.MatchedBy(func(c *companydomain.Company) bool {
		return c.ID == id && c.Name == "Acme"
	})).Return(nil).Once()
	events.On("Publish", ctx, mock.MatchedBy(func(event appcompany.Event) bool {
		return event.Type == appcompany.EventCompanyUpdated && event.CompanyID == id
	})).Return(nil).Once()

	updated, err := service.PatchCompany(ctx, id, appcompany.PatchInput{Name: &name})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "Acme" {
		t.Fatalf("expected trimmed patched name, got %q", updated.Name)
	}
}

func TestPatchCompanyReturnsEmptyPatch(t *testing.T) {
	ctx := context.Background()
	repo := mocks.NewRepository(t)
	events := mocks.NewEventProducer(t)
	service := appcompany.NewService(repo, events, &testLogger)

	_, err := service.PatchCompany(ctx, uuid.New(), appcompany.PatchInput{})
	if !errors.Is(err, companydomain.ErrEmptyPatch) {
		t.Fatalf("expected ErrEmptyPatch, got %v", err)
	}
	repo.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything)
	events.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything)
}

func TestPatchCompanyReturnsGetByIDError(t *testing.T) {
	ctx := context.Background()
	id := uuid.New()
	repo := mocks.NewRepository(t)
	events := mocks.NewEventProducer(t)
	service := appcompany.NewService(repo, events, &testLogger)
	getErr := errors.New("get failed")
	description := "updated"

	repo.On("GetByID", ctx, id).Return((*companydomain.Company)(nil), getErr).Once()

	_, err := service.PatchCompany(ctx, id, appcompany.PatchInput{Description: &description})
	if !errors.Is(err, getErr) {
		t.Fatalf("expected get error, got %v", err)
	}
	repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
	events.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything)
}

func TestPatchCompanyReturnsDuplicateName(t *testing.T) {
	ctx := context.Background()
	id := uuid.New()
	repo := mocks.NewRepository(t)
	events := mocks.NewEventProducer(t)
	service := appcompany.NewService(repo, events, &testLogger)
	existing, newErr := companydomain.New("Acme", "description", 10, true, companydomain.TypeCorporations)
	if newErr != nil {
		t.Fatal(newErr)
	}
	existing.ID = id
	other, newErr := companydomain.New("Other", "description", 10, true, companydomain.TypeCorporations)
	if newErr != nil {
		t.Fatal(newErr)
	}
	other.ID = uuid.New()
	name := "Other"

	repo.On("GetByID", ctx, id).Return(existing, nil).Once()
	repo.On("GetByName", ctx, "Other").Return(other, nil).Once()

	_, err := service.PatchCompany(ctx, id, appcompany.PatchInput{Name: &name})
	if !errors.Is(err, companydomain.ErrDuplicateName) {
		t.Fatalf("expected ErrDuplicateName, got %v", err)
	}
	repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
	events.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything)
}

func TestPatchCompanyReturnsNameLookupError(t *testing.T) {
	ctx := context.Background()
	id := uuid.New()
	repo := mocks.NewRepository(t)
	events := mocks.NewEventProducer(t)
	service := appcompany.NewService(repo, events, &testLogger)
	existing, newErr := companydomain.New("Acme", "description", 10, true, companydomain.TypeCorporations)
	if newErr != nil {
		t.Fatal(newErr)
	}
	existing.ID = id
	name := "Other"
	lookupErr := errors.New("lookup failed")

	repo.On("GetByID", ctx, id).Return(existing, nil).Once()
	repo.On("GetByName", ctx, "Other").Return((*companydomain.Company)(nil), lookupErr).Once()

	_, err := service.PatchCompany(ctx, id, appcompany.PatchInput{Name: &name})
	if !errors.Is(err, lookupErr) {
		t.Fatalf("expected lookup error, got %v", err)
	}
	repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
	events.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything)
}

func TestPatchCompanyReturnsValidationError(t *testing.T) {
	ctx := context.Background()
	id := uuid.New()
	repo := mocks.NewRepository(t)
	events := mocks.NewEventProducer(t)
	service := appcompany.NewService(repo, events, &testLogger)
	existing, newErr := companydomain.New("Acme", "description", 10, true, companydomain.TypeCorporations)
	if newErr != nil {
		t.Fatal(newErr)
	}
	existing.ID = id
	employees := -1

	repo.On("GetByID", ctx, id).Return(existing, nil).Once()

	_, err := service.PatchCompany(ctx, id, appcompany.PatchInput{AmountOfEmployees: &employees})
	if err == nil {
		t.Fatal("expected validation error")
	}

	var validationErr companydomain.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
	events.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything)
}

func TestPatchCompanyReturnsUpdateError(t *testing.T) {
	ctx := context.Background()
	id := uuid.New()
	repo := mocks.NewRepository(t)
	events := mocks.NewEventProducer(t)
	service := appcompany.NewService(repo, events, &testLogger)
	existing, newErr := companydomain.New("Acme", "description", 10, true, companydomain.TypeCorporations)
	if newErr != nil {
		t.Fatal(newErr)
	}
	existing.ID = id
	description := "updated"
	updateErr := errors.New("update failed")

	repo.On("GetByID", ctx, id).Return(existing, nil).Once()
	repo.On("Update", ctx, mock.AnythingOfType("*company.Company")).Return(updateErr).Once()

	_, err := service.PatchCompany(ctx, id, appcompany.PatchInput{Description: &description})
	if !errors.Is(err, updateErr) {
		t.Fatalf("expected update error, got %v", err)
	}
	events.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything)
}

func TestPatchCompanyReturnsPublishError(t *testing.T) {
	ctx := context.Background()
	id := uuid.New()
	repo := mocks.NewRepository(t)
	events := mocks.NewEventProducer(t)
	service := appcompany.NewService(repo, events, &testLogger)
	existing, newErr := companydomain.New("Acme", "description", 10, true, companydomain.TypeCorporations)
	if newErr != nil {
		t.Fatal(newErr)
	}
	existing.ID = id
	description := "updated"
	publishErr := errors.New("publish failed")

	repo.On("GetByID", ctx, id).Return(existing, nil).Once()
	repo.On("Update", ctx, mock.AnythingOfType("*company.Company")).Return(nil).Once()
	events.On("Publish", ctx, mock.MatchedBy(func(event appcompany.Event) bool {
		return event.Type == appcompany.EventCompanyUpdated && event.CompanyID == id
	})).Return(publishErr).Once()

	_, err := service.PatchCompany(ctx, id, appcompany.PatchInput{Description: &description})
	if err == nil {
		t.Fatal("expected publish error")
	}
	if !errors.Is(err, publishErr) {
		t.Fatalf("expected wrapped publish error, got %v", err)
	}
	if !strings.Contains(err.Error(), "publish company event") {
		t.Fatalf("expected publish error context, got %v", err)
	}
}

func TestDeleteCompanySuccess(t *testing.T) {
	ctx := context.Background()
	id := uuid.New()
	repo := mocks.NewRepository(t)
	events := mocks.NewEventProducer(t)
	service := appcompany.NewService(repo, events, &testLogger)
	existing, newErr := companydomain.New("Acme", "description", 10, true, companydomain.TypeCorporations)
	if newErr != nil {
		t.Fatal(newErr)
	}
	existing.ID = id

	repo.On("GetByID", ctx, id).Return(existing, nil).Once()
	repo.On("Delete", ctx, id).Return(nil).Once()
	events.On("Publish", ctx, mock.MatchedBy(func(event appcompany.Event) bool {
		return event.ID != uuid.Nil &&
			event.Type == appcompany.EventCompanyDeleted &&
			event.CompanyID == id &&
			event.Company == existing &&
			!event.OccurredAt.IsZero()
	})).Return(nil).Once()

	if err := service.DeleteCompany(ctx, id); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteCompanyReturnsGetByIDError(t *testing.T) {
	ctx := context.Background()
	id := uuid.New()
	repo := mocks.NewRepository(t)
	events := mocks.NewEventProducer(t)
	service := appcompany.NewService(repo, events, &testLogger)
	getErr := errors.New("get failed")

	repo.On("GetByID", ctx, id).Return((*companydomain.Company)(nil), getErr).Once()

	err := service.DeleteCompany(ctx, id)
	if !errors.Is(err, getErr) {
		t.Fatalf("expected get error, got %v", err)
	}
	repo.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)
	events.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything)
}

func TestDeleteCompanyReturnsDeleteError(t *testing.T) {
	ctx := context.Background()
	id := uuid.New()
	repo := mocks.NewRepository(t)
	events := mocks.NewEventProducer(t)
	service := appcompany.NewService(repo, events, &testLogger)
	existing, newErr := companydomain.New("Acme", "description", 10, true, companydomain.TypeCorporations)
	if newErr != nil {
		t.Fatal(newErr)
	}
	existing.ID = id
	deleteErr := errors.New("delete failed")

	repo.On("GetByID", ctx, id).Return(existing, nil).Once()
	repo.On("Delete", ctx, id).Return(deleteErr).Once()

	err := service.DeleteCompany(ctx, id)
	if !errors.Is(err, deleteErr) {
		t.Fatalf("expected delete error, got %v", err)
	}
	events.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything)
}

func TestDeleteCompanyReturnsPublishError(t *testing.T) {
	ctx := context.Background()
	id := uuid.New()
	repo := mocks.NewRepository(t)
	events := mocks.NewEventProducer(t)
	service := appcompany.NewService(repo, events, &testLogger)
	existing, newErr := companydomain.New("Acme", "description", 10, true, companydomain.TypeCorporations)
	if newErr != nil {
		t.Fatal(newErr)
	}
	existing.ID = id
	publishErr := errors.New("publish failed")

	repo.On("GetByID", ctx, id).Return(existing, nil).Once()
	repo.On("Delete", ctx, id).Return(nil).Once()
	events.On("Publish", ctx, mock.MatchedBy(func(event appcompany.Event) bool {
		return event.Type == appcompany.EventCompanyDeleted && event.CompanyID == id
	})).Return(publishErr).Once()

	err := service.DeleteCompany(ctx, id)
	if err == nil {
		t.Fatal("expected publish error")
	}
	if !errors.Is(err, publishErr) {
		t.Fatalf("expected wrapped publish error, got %v", err)
	}
	if !strings.Contains(err.Error(), "publish company event") {
		t.Fatalf("expected publish error context, got %v", err)
	}
}

func TestGetCompanyReturnsCompany(t *testing.T) {
	ctx := context.Background()
	id := uuid.New()
	repo := mocks.NewRepository(t)
	events := mocks.NewEventProducer(t)
	service := appcompany.NewService(repo, events, &testLogger)
	existing, newErr := companydomain.New("Acme", "description", 10, true, companydomain.TypeCorporations)
	if newErr != nil {
		t.Fatal(newErr)
	}
	existing.ID = id

	repo.On("GetByID", ctx, id).Return(existing, nil).Once()

	got, err := service.GetCompany(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if got != existing {
		t.Fatal("expected repository company")
	}
	events.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything)
}

func TestGetCompanyReturnsRepositoryError(t *testing.T) {
	ctx := context.Background()
	id := uuid.New()
	repo := mocks.NewRepository(t)
	events := mocks.NewEventProducer(t)
	service := appcompany.NewService(repo, events, &testLogger)
	getErr := errors.New("get failed")

	repo.On("GetByID", ctx, id).Return((*companydomain.Company)(nil), getErr).Once()

	_, err := service.GetCompany(ctx, id)
	if !errors.Is(err, getErr) {
		t.Fatalf("expected get error, got %v", err)
	}
	events.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything)
}

func validCreateInput(name string) appcompany.CreateInput {
	return appcompany.CreateInput{
		Name:              name,
		Description:       "description",
		AmountOfEmployees: 10,
		Registered:        true,
		Type:              companydomain.TypeCorporations,
	}
}
