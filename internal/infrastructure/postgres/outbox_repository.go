package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	appcompany "company-service/internal/application/company"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v4/pgxpool"
)

type OutboxRepository struct {
	pool *pgxpool.Pool
}

func NewOutboxRepository(pool *pgxpool.Pool) *OutboxRepository {
	return &OutboxRepository{pool: pool}
}

func (r *OutboxRepository) Store(ctx context.Context, event appcompany.Event) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal outbox event: %w", err)
	}

	query, args, err := psql.Insert("outbox_events").
		Columns("id", "event_type", "aggregate_id", "payload", "created_at").
		Values(event.ID, event.Type, event.CompanyID, payload, event.OccurredAt).
		ToSql()
	if err != nil {
		return fmt.Errorf("build store outbox event query: %w", err)
	}

	if _, err := executorFromContext(ctx, r.pool).Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("store outbox event: %w", err)
	}

	return nil
}

func (r *OutboxRepository) FetchPending(ctx context.Context, limit uint64) ([]appcompany.OutboxEvent, error) {
	query, args, err := psql.Select("id", "payload", "attempts").
		From("outbox_events").
		Where("published_at IS NULL").
		Where("available_at <= now()").
		OrderBy("created_at ASC").
		Limit(limit).
		Suffix("FOR UPDATE SKIP LOCKED").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build fetch pending outbox events query: %w", err)
	}

	rows, err := executorFromContext(ctx, r.pool).Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("fetch pending outbox events: %w", err)
	}
	defer rows.Close()

	events := make([]appcompany.OutboxEvent, 0)
	for rows.Next() {
		var event appcompany.OutboxEvent
		var payload []byte
		if err := rows.Scan(&event.ID, &payload, &event.Attempts); err != nil {
			return nil, fmt.Errorf("scan outbox event: %w", err)
		}
		if err := json.Unmarshal(payload, &event.Event); err != nil {
			return nil, fmt.Errorf("unmarshal outbox event payload: %w", err)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate outbox events: %w", err)
	}

	return events, nil
}

func (r *OutboxRepository) MarkPublished(ctx context.Context, id uuid.UUID) error {
	query, args, err := psql.Update("outbox_events").
		Set("published_at", sq.Expr("now()")).
		Set("last_error", nil).
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build mark outbox event published query: %w", err)
	}

	if _, err := executorFromContext(ctx, r.pool).Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("mark outbox event published: %w", err)
	}

	return nil
}

func (r *OutboxRepository) MarkFailed(ctx context.Context, id uuid.UUID, publishErr error) error {
	query, args, err := psql.Update("outbox_events").
		Set("attempts", sq.Expr("attempts + 1")).
		Set("last_error", publishErr.Error()).
		Set("available_at", sq.Expr("now() + interval '10 seconds'")).
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build mark outbox event failed query: %w", err)
	}

	if _, err := executorFromContext(ctx, r.pool).Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("mark outbox event failed: %w", err)
	}

	return nil
}
