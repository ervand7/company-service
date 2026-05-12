package http

import (
	"errors"
	"testing"
	"time"

	companydomain "company-service/internal/domain/company"

	"github.com/google/uuid"
)

func TestCreateCompanyRequestToInput(t *testing.T) {
	id := uuid.New()
	name := "Acme"
	description := "description"
	employees := 10
	registered := true
	companyType := string(companydomain.TypeCorporations)

	input, err := (createCompanyRequest{
		ID:                stringPtr(id.String()),
		Name:              &name,
		Description:       &description,
		AmountOfEmployees: &employees,
		Registered:        &registered,
		Type:              &companyType,
	}).toInput()
	if err != nil {
		t.Fatal(err)
	}

	if input.ID != id {
		t.Fatalf("expected id %q, got %q", id, input.ID)
	}
	if input.Name != name {
		t.Fatalf("expected name %q, got %q", name, input.Name)
	}
	if input.Description != description {
		t.Fatalf("expected description %q, got %q", description, input.Description)
	}
	if input.AmountOfEmployees != employees {
		t.Fatalf("expected employees %d, got %d", employees, input.AmountOfEmployees)
	}
	if input.Registered != registered {
		t.Fatalf("expected registered %v, got %v", registered, input.Registered)
	}
	if input.Type != companydomain.TypeCorporations {
		t.Fatalf("expected type %q, got %q", companydomain.TypeCorporations, input.Type)
	}
}

func TestCreateCompanyRequestToInputDefaultsMissingDescription(t *testing.T) {
	id := uuid.NewString()
	name := "Acme"
	employees := 10
	registered := false
	companyType := string(companydomain.TypeNonProfit)

	input, err := (createCompanyRequest{
		ID:                &id,
		Name:              &name,
		AmountOfEmployees: &employees,
		Registered:        &registered,
		Type:              &companyType,
	}).toInput()
	if err != nil {
		t.Fatal(err)
	}

	if input.Description != "" {
		t.Fatalf("expected empty description, got %q", input.Description)
	}
	if input.Registered {
		t.Fatal("expected registered false value to be preserved")
	}
}

func TestCreateCompanyRequestToInputRequiresFields(t *testing.T) {
	_, err := (createCompanyRequest{}).toInput()
	if err == nil {
		t.Fatal("expected validation error")
	}

	var validationErr companydomain.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	for _, field := range []string{"id", "name", "amount_of_employees", "registered", "type"} {
		if validationErr.Fields[field] != "is required" {
			t.Fatalf("expected required validation for %s, got %q", field, validationErr.Fields[field])
		}
	}
}

func TestCreateCompanyRequestToInputRejectsInvalidID(t *testing.T) {
	id := "not-a-uuid"
	name := "Acme"
	employees := 10
	registered := true
	companyType := string(companydomain.TypeCorporations)

	_, err := (createCompanyRequest{
		ID:                &id,
		Name:              &name,
		AmountOfEmployees: &employees,
		Registered:        &registered,
		Type:              &companyType,
	}).toInput()
	if err == nil {
		t.Fatal("expected validation error")
	}

	var validationErr companydomain.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if validationErr.Fields["id"] != "must be a valid uuid" {
		t.Fatalf("expected invalid id validation, got %q", validationErr.Fields["id"])
	}
}

func TestPatchCompanyRequestToInput(t *testing.T) {
	name := "Acme"
	description := "updated"
	employees := 20
	registered := false
	companyType := string(companydomain.TypeCooperative)

	input, err := (patchCompanyRequest{
		Name:              &name,
		Description:       &description,
		AmountOfEmployees: &employees,
		Registered:        &registered,
		Type:              &companyType,
	}).toInput()
	if err != nil {
		t.Fatal(err)
	}

	if input.Name != &name {
		t.Fatal("expected name pointer to be preserved")
	}
	if input.Description != &description {
		t.Fatal("expected description pointer to be preserved")
	}
	if input.AmountOfEmployees != &employees {
		t.Fatal("expected employees pointer to be preserved")
	}
	if input.Registered != &registered {
		t.Fatal("expected registered pointer to be preserved")
	}
	if input.Type == nil || *input.Type != companydomain.TypeCooperative {
		t.Fatalf("expected type %q, got %v", companydomain.TypeCooperative, input.Type)
	}
}

func TestPatchCompanyRequestToInputAllowsEmptyPatch(t *testing.T) {
	input, err := (patchCompanyRequest{}).toInput()
	if err != nil {
		t.Fatal(err)
	}

	if input.Name != nil ||
		input.Description != nil ||
		input.AmountOfEmployees != nil ||
		input.Registered != nil ||
		input.Type != nil {
		t.Fatalf("expected empty patch input, got %+v", input)
	}
}

func TestToCompanyResponse(t *testing.T) {
	id := uuid.New()
	createdAt := time.Date(2026, 5, 12, 9, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Hour)

	response := toCompanyResponse(&companydomain.Company{
		ID:                id,
		Name:              "Acme",
		Description:       "description",
		AmountOfEmployees: 10,
		Registered:        true,
		Type:              companydomain.TypeSoleProprietorship,
		CreatedAt:         createdAt,
		UpdatedAt:         updatedAt,
	})

	if response.ID != id.String() {
		t.Fatalf("expected id %q, got %q", id.String(), response.ID)
	}
	if response.Name != "Acme" {
		t.Fatalf("expected name %q, got %q", "Acme", response.Name)
	}
	if response.Description != "description" {
		t.Fatalf("expected description %q, got %q", "description", response.Description)
	}
	if response.AmountOfEmployees != 10 {
		t.Fatalf("expected employees %d, got %d", 10, response.AmountOfEmployees)
	}
	if !response.Registered {
		t.Fatal("expected registered true")
	}
	if response.Type != string(companydomain.TypeSoleProprietorship) {
		t.Fatalf("expected type %q, got %q", companydomain.TypeSoleProprietorship, response.Type)
	}
	if !response.CreatedAt.Equal(createdAt) {
		t.Fatalf("expected created at %v, got %v", createdAt, response.CreatedAt)
	}
	if !response.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("expected updated at %v, got %v", updatedAt, response.UpdatedAt)
	}
}

func stringPtr(value string) *string {
	return &value
}
