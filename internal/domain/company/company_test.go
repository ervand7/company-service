package company_test

import (
	"errors"
	"testing"

	"company-service/internal/domain/company"

	"github.com/google/uuid"
)

func TestNewValidatesRequiredFields(t *testing.T) {
	_, err := company.NewWithID(uuid.Nil, "", "", -1, true, "Invalid")
	if err == nil {
		t.Fatal("expected validation error")
	}

	var validationErr company.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	for _, field := range []string{"id", "name", "amount_of_employees", "type"} {
		if validationErr.Fields[field] == "" {
			t.Fatalf("expected validation error for %s", field)
		}
	}
}

func TestNewTrimsAndValidatesLengths(t *testing.T) {
	_, err := company.New("  Acme  ", string(make([]byte, company.MaxDescriptionLength+1)), 0, false, company.TypeNonProfit)
	if err == nil {
		t.Fatal("expected validation error")
	}

	var validationErr company.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if validationErr.Fields["description"] == "" {
		t.Fatal("expected description validation error")
	}
}

func TestApplyPatchPreservesMissingFields(t *testing.T) {
	c, err := company.New("Acme", "original", 10, true, company.TypeCorporations)
	if err != nil {
		t.Fatal(err)
	}

	registered := false
	err = c.ApplyPatch(company.Patch{Registered: &registered})
	if err != nil {
		t.Fatal(err)
	}

	if c.Name != "Acme" {
		t.Fatalf("expected name to be preserved, got %q", c.Name)
	}
	if c.Description != "original" {
		t.Fatalf("expected description to be preserved, got %q", c.Description)
	}
	if c.AmountOfEmployees != 10 {
		t.Fatalf("expected employees to be preserved, got %d", c.AmountOfEmployees)
	}
	if c.Registered {
		t.Fatal("expected registered to be patched to false")
	}
}

func TestApplyPatchAllowsZeroValues(t *testing.T) {
	c, err := company.New("Acme", "original", 10, true, company.TypeCorporations)
	if err != nil {
		t.Fatal(err)
	}

	employees := 0
	err = c.ApplyPatch(company.Patch{AmountOfEmployees: &employees})
	if err != nil {
		t.Fatal(err)
	}

	if c.AmountOfEmployees != 0 {
		t.Fatalf("expected employees to be 0, got %d", c.AmountOfEmployees)
	}
}

func TestApplyPatchRejectsEmptyPatch(t *testing.T) {
	c, err := company.New("Acme", "", 1, true, company.TypeCooperative)
	if err != nil {
		t.Fatal(err)
	}

	err = c.ApplyPatch(company.Patch{})
	if !errors.Is(err, company.ErrEmptyPatch) {
		t.Fatalf("expected ErrEmptyPatch, got %v", err)
	}
}
