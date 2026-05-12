package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	companydomain "company-service/internal/domain/company"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

type rowScanner interface {
	Scan(dest ...any) error
}

const uniqueViolationCode = "23505"

var psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

type CompanyRepository struct {
	pool *pgxpool.Pool
}

func NewCompanyRepository(pool *pgxpool.Pool) *CompanyRepository {
	return &CompanyRepository{pool: pool}
}

func (r *CompanyRepository) Create(ctx context.Context, company *companydomain.Company) error {
	query, args, err := psql.Insert("companies").
		Columns(
			"id",
			"name",
			"description",
			"amount_of_employees",
			"registered",
			"type",
			"created_at",
			"updated_at",
		).
		Values(
			company.ID,
			company.Name,
			company.Description,
			company.AmountOfEmployees,
			company.Registered,
			company.Type,
			company.CreatedAt,
			company.UpdatedAt,
		).
		ToSql()
	if err != nil {
		return fmt.Errorf("build create company query: %w", err)
	}

	_, err = r.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("create company: %w", mapPostgresError(err))
	}

	return nil
}

func (r *CompanyRepository) Update(ctx context.Context, company *companydomain.Company) error {
	query, args, err := psql.Update("companies").
		Set("name", company.Name).
		Set("description", company.Description).
		Set("amount_of_employees", company.AmountOfEmployees).
		Set("registered", company.Registered).
		Set("type", company.Type).
		Set("updated_at", company.UpdatedAt).
		Where(sq.Eq{"id": company.ID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build update company query: %w", err)
	}

	tag, err := r.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update company: %w", mapPostgresError(err))
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("update company: %w", companydomain.ErrNotFound)
	}

	return nil
}

func (r *CompanyRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query, args, err := psql.Delete("companies").
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build delete company query: %w", err)
	}

	tag, err := r.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("delete company: %w", mapPostgresError(err))
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("delete company: %w", companydomain.ErrNotFound)
	}

	return nil
}

func (r *CompanyRepository) GetByID(ctx context.Context, id uuid.UUID) (*companydomain.Company, error) {
	query, args, err := selectCompany().
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build get company by id query: %w", err)
	}

	company, err := scanCompany(r.pool.QueryRow(ctx, query, args...))
	if err != nil {
		return nil, fmt.Errorf("get company by id: %w", err)
	}

	return company, nil
}

func (r *CompanyRepository) GetByName(ctx context.Context, name string) (*companydomain.Company, error) {
	query, args, err := selectCompany().
		Where(sq.Eq{"name": name}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build get company by name query: %w", err)
	}

	company, err := scanCompany(r.pool.QueryRow(ctx, query, args...))
	if err != nil {
		return nil, fmt.Errorf("get company by name: %w", err)
	}

	return company, nil
}

func selectCompany() sq.SelectBuilder {
	return psql.Select(
		"id",
		"name",
		"description",
		"amount_of_employees",
		"registered",
		"type",
		"created_at",
		"updated_at",
	).From("companies")
}

func scanCompany(row rowScanner) (*companydomain.Company, error) {
	var c companydomain.Company
	var companyType string
	var createdAt time.Time
	var updatedAt time.Time

	err := row.Scan(
		&c.ID,
		&c.Name,
		&c.Description,
		&c.AmountOfEmployees,
		&c.Registered,
		&companyType,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("scan company: %w", companydomain.ErrNotFound)
		}
		return nil, fmt.Errorf("scan company: %w", err)
	}

	c.Type = companydomain.CompanyType(companyType)
	c.CreatedAt = createdAt.UTC()
	c.UpdatedAt = updatedAt.UTC()

	return &c, nil
}

func mapPostgresError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolationCode {
		return fmt.Errorf("map postgres error: %w", companydomain.ErrDuplicateName)
	}

	return fmt.Errorf("map postgres error: %w", err)
}
