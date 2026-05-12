package http

import (
	"time"

	appcompany "company-service/internal/application/company"
	companydomain "company-service/internal/domain/company"
)

type createCompanyRequest struct {
	Name              *string `json:"name"`
	Description       *string `json:"description"`
	AmountOfEmployees *int    `json:"amount_of_employees"`
	Registered        *bool   `json:"registered"`
	Type              *string `json:"type"`
}

func (r createCompanyRequest) toInput() (appcompany.CreateInput, error) {
	fields := make(map[string]string)
	if r.Name == nil {
		fields["name"] = "is required"
	}
	if r.AmountOfEmployees == nil {
		fields["amount_of_employees"] = "is required"
	}
	if r.Registered == nil {
		fields["registered"] = "is required"
	}
	if r.Type == nil {
		fields["type"] = "is required"
	}
	if err := companydomain.NewValidationError(fields); err != nil {
		return appcompany.CreateInput{}, err
	}

	description := ""
	if r.Description != nil {
		description = *r.Description
	}

	return appcompany.CreateInput{
		Name:              *r.Name,
		Description:       description,
		AmountOfEmployees: *r.AmountOfEmployees,
		Registered:        *r.Registered,
		Type:              companydomain.CompanyType(*r.Type),
	}, nil
}

type patchCompanyRequest struct {
	Name              *string `json:"name"`
	Description       *string `json:"description"`
	AmountOfEmployees *int    `json:"amount_of_employees"`
	Registered        *bool   `json:"registered"`
	Type              *string `json:"type"`
}

func (r patchCompanyRequest) toInput() (appcompany.PatchInput, error) {
	var companyType *companydomain.CompanyType
	if r.Type != nil {
		t := companydomain.CompanyType(*r.Type)
		companyType = &t
	}

	return appcompany.PatchInput{
		Name:              r.Name,
		Description:       r.Description,
		AmountOfEmployees: r.AmountOfEmployees,
		Registered:        r.Registered,
		Type:              companyType,
	}, nil
}

type companyResponse struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	Description       string    `json:"description"`
	AmountOfEmployees int       `json:"amount_of_employees"`
	Registered        bool      `json:"registered"`
	Type              string    `json:"type"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

func toCompanyResponse(c *companydomain.Company) companyResponse {
	return companyResponse{
		ID:                c.ID.String(),
		Name:              c.Name,
		Description:       c.Description,
		AmountOfEmployees: c.AmountOfEmployees,
		Registered:        c.Registered,
		Type:              string(c.Type),
		CreatedAt:         c.CreatedAt,
		UpdatedAt:         c.UpdatedAt,
	}
}
