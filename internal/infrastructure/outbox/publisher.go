package outbox

import (
	"context"
	"fmt"
	"time"

	appcompany "company-service/internal/application/company"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

//go:generate go run github.com/vektra/mockery/v2@v2.53.6 --dir . --name Repository --output mocks --outpkg mocks --case underscore
type Repository interface {
	FetchPending(ctx context.Context, limit uint64) ([]appcompany.OutboxEvent, error)
	MarkPublished(ctx context.Context, id uuid.UUID) error
	MarkFailed(ctx context.Context, id uuid.UUID, err error) error
}

//go:generate go run github.com/vektra/mockery/v2@v2.53.6 --dir . --name TransactionRunner --output mocks --outpkg mocks --case underscore
type TransactionRunner interface {
	WithinTransaction(ctx context.Context, fn func(context.Context) error) error
}

type Publisher struct {
	repo     Repository
	tx       TransactionRunner
	producer appcompany.EventProducer
	interval time.Duration
	batch    uint64
	logger   zerolog.Logger
}

func NewPublisher(repo Repository, tx TransactionRunner, producer appcompany.EventProducer, interval time.Duration, batchSize int, logger zerolog.Logger) *Publisher {
	if interval <= 0 {
		interval = 2 * time.Second
	}
	if batchSize <= 0 {
		batchSize = 10
	}

	return &Publisher{
		repo:     repo,
		tx:       tx,
		producer: producer,
		interval: interval,
		batch:    uint64(batchSize),
		logger:   logger,
	}
}

func (p *Publisher) Run(ctx context.Context) {
	p.processAndLog(ctx)

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.logger.Info().Msg("outbox publisher stopped")
			return
		case <-ticker.C:
			p.processAndLog(ctx)
		}
	}
}

func (p *Publisher) ProcessBatch(ctx context.Context) error {
	if p.tx == nil {
		return p.processBatch(ctx)
	}
	return p.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		return p.processBatch(ctx)
	})
}

func (p *Publisher) processBatch(ctx context.Context) error {
	events, err := p.repo.FetchPending(ctx, p.batch)
	if err != nil {
		return err
	}

	for _, event := range events {
		if err := p.producer.Publish(ctx, event.Event); err != nil {
			if markErr := p.repo.MarkFailed(ctx, event.ID, err); markErr != nil {
				return fmt.Errorf("mark outbox event failed after publish error %v: %w", err, markErr)
			}
			p.logger.Error().
				Err(err).
				Str("outbox_event_id", event.ID.String()).
				Str("event_type", event.Event.Type).
				Int("attempts", event.Attempts+1).
				Msg("failed to publish outbox event")
			continue
		}

		if err := p.repo.MarkPublished(ctx, event.ID); err != nil {
			return err
		}
	}

	return nil
}

func (p *Publisher) processAndLog(ctx context.Context) {
	if err := p.ProcessBatch(ctx); err != nil {
		p.logger.Error().Err(err).Msg("outbox publisher batch failed")
	}
}
