package company

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	companydomain "company-service/internal/domain/company"

	"github.com/google/uuid"
)

type Service struct {
	repo    Repository
	events  EventProducer
	logger  Logger
	now     func() time.Time
	newUUID func() uuid.UUID
}

func NewService(repo Repository, events EventProducer, logger Logger) *Service {
	return &Service{
		repo:    repo,
		events:  events,
		logger:  logger,
		now:     func() time.Time { return time.Now().UTC() },
		newUUID: uuid.New,
	}
}

func (s *Service) CreateCompany(ctx context.Context, input CreateInput) (*companydomain.Company, error) {
	company, err := companydomain.New(input.Name, input.Description, input.AmountOfEmployees, input.Registered, input.Type)
	if err != nil {
		return nil, fmt.Errorf("create company: %w", err)
	}

	if _, err := s.repo.GetByName(ctx, company.Name); err == nil {
		return nil, fmt.Errorf("create company: %w", companydomain.ErrDuplicateName)
	} else if !errors.Is(err, companydomain.ErrNotFound) {
		return nil, fmt.Errorf("get company by name: %w", err)
	}

	if err := s.repo.Create(ctx, company); err != nil {
		return nil, fmt.Errorf("create company: %w", err)
	}

	if err := s.publish(ctx, EventCompanyCreated, company); err != nil {
		return nil, fmt.Errorf("publish creating company: %w", err)
	}

	return company, nil
}

func (s *Service) PatchCompany(ctx context.Context, id uuid.UUID, input PatchInput) (*companydomain.Company, error) {
	patch := companydomain.Patch{
		Name:              input.Name,
		Description:       input.Description,
		AmountOfEmployees: input.AmountOfEmployees,
		Registered:        input.Registered,
		Type:              input.Type,
	}
	if patch.IsEmpty() {
		return nil, fmt.Errorf("patch company: %w", companydomain.ErrEmptyPatch)
	}

	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get company by id: %w", err)
	}

	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if found, err := s.repo.GetByName(ctx, name); err == nil && found.ID != id {
			return nil, fmt.Errorf("patch company: %w", companydomain.ErrDuplicateName)
		} else if err != nil && !errors.Is(err, companydomain.ErrNotFound) {
			return nil, fmt.Errorf("get company by name: %w", err)
		}
	}

	if err := existing.ApplyPatch(patch); err != nil {
		return nil, fmt.Errorf("apply company patch: %w", err)
	}

	if err := s.repo.Update(ctx, existing); err != nil {
		return nil, fmt.Errorf("update company: %w", err)
	}

	if err := s.publish(ctx, EventCompanyUpdated, existing); err != nil {
		return nil, fmt.Errorf("patch company: %w", err)
	}

	return existing, nil
}

func (s *Service) DeleteCompany(ctx context.Context, id uuid.UUID) error {
	company, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("get company by id: %w", err)
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete company: %w", err)
	}

	if err := s.publish(ctx, EventCompanyDeleted, company); err != nil {
		return fmt.Errorf("publish deleting company: %w", err)
	}

	return nil
}

func (s *Service) GetCompany(ctx context.Context, id uuid.UUID) (*companydomain.Company, error) {
	company, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get company by id: %w", err)
	}

	return company, nil
}

func (s *Service) publish(ctx context.Context, eventType string, company *companydomain.Company) error {
	event := Event{
		ID:         s.newUUID(),
		Type:       eventType,
		OccurredAt: s.now(),
		CompanyID:  company.ID,
		Company:    company,
	}

	if err := s.events.Publish(ctx, event); err != nil {
		return fmt.Errorf("publish company event: %w", err)
	}

	return nil
}
