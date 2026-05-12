package company

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	MaxNameLength        = 15
	MaxDescriptionLength = 3000
)

type Company struct {
	ID                uuid.UUID
	Name              string
	Description       string
	AmountOfEmployees int
	Registered        bool
	Type              CompanyType
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type Patch struct {
	Name              *string
	Description       *string
	AmountOfEmployees *int
	Registered        *bool
	Type              *CompanyType
}

func New(name string, description string, amountOfEmployees int, registered bool, companyType CompanyType) (*Company, error) {
	now := time.Now().UTC()
	c := &Company{
		ID:                uuid.New(),
		Name:              strings.TrimSpace(name),
		Description:       strings.TrimSpace(description),
		AmountOfEmployees: amountOfEmployees,
		Registered:        registered,
		Type:              companyType,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	if err := c.Validate(); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *Company) Validate() error {
	fields := make(map[string]string)

	if c.ID == uuid.Nil {
		fields["id"] = "is required"
	}
	validateName(c.Name, fields)
	validateDescription(c.Description, fields)
	validateAmountOfEmployees(c.AmountOfEmployees, fields)
	validateType(c.Type, fields)

	return NewValidationError(fields)
}

func (c *Company) ApplyPatch(p Patch) error {
	if p.IsEmpty() {
		return ErrEmptyPatch
	}

	if p.Name != nil {
		name := strings.TrimSpace(*p.Name)
		fields := make(map[string]string)
		validateName(name, fields)
		if err := NewValidationError(fields); err != nil {
			return err
		}
		c.Name = name
	}

	if p.Description != nil {
		description := strings.TrimSpace(*p.Description)
		fields := make(map[string]string)
		validateDescription(description, fields)
		if err := NewValidationError(fields); err != nil {
			return err
		}
		c.Description = description
	}

	if p.AmountOfEmployees != nil {
		fields := make(map[string]string)
		validateAmountOfEmployees(*p.AmountOfEmployees, fields)
		if err := NewValidationError(fields); err != nil {
			return err
		}
		c.AmountOfEmployees = *p.AmountOfEmployees
	}

	if p.Registered != nil {
		c.Registered = *p.Registered
	}

	if p.Type != nil {
		fields := make(map[string]string)
		validateType(*p.Type, fields)
		if err := NewValidationError(fields); err != nil {
			return err
		}
		c.Type = *p.Type
	}

	c.UpdatedAt = time.Now().UTC()

	return c.Validate()
}

func (p Patch) IsEmpty() bool {
	return p.Name == nil &&
		p.Description == nil &&
		p.AmountOfEmployees == nil &&
		p.Registered == nil &&
		p.Type == nil
}

func validateName(name string, fields map[string]string) {
	if strings.TrimSpace(name) == "" {
		fields["name"] = "is required"
		return
	}
	if utf8.RuneCountInString(name) > MaxNameLength {
		fields["name"] = "must be at most 15 characters"
	}
}

func validateDescription(description string, fields map[string]string) {
	if utf8.RuneCountInString(description) > MaxDescriptionLength {
		fields["description"] = "must be at most 3000 characters"
	}
}

func validateAmountOfEmployees(amount int, fields map[string]string) {
	if amount < 0 {
		fields["amount_of_employees"] = "must be greater than or equal to 0"
	}
}

func validateType(companyType CompanyType, fields map[string]string) {
	if !companyType.IsValid() {
		fields["type"] = "must be one of Corporations, NonProfit, Cooperative, Sole Proprietorship"
	}
}
