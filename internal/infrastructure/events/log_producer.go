package events

import (
	"context"

	appcompany "company-service/internal/application/company"

	"github.com/rs/zerolog"
)

type LogProducer struct {
	logger zerolog.Logger
}

func NewLogProducer(logger zerolog.Logger) *LogProducer {
	return &LogProducer{logger: logger}
}

func (p *LogProducer) Publish(ctx context.Context, event appcompany.Event) error {
	_ = ctx
	logger := p.logger.Info().
		Str("event_id", event.ID.String()).
		Str("event_type", event.Type).
		Time("occurred_at", event.OccurredAt).
		Str("company_id", event.CompanyID.String())
	if event.Company != nil {
		logger = logger.Str("company_name", event.Company.Name).Str("company_type", string(event.Company.Type))
	}

	logger.Msg("company event published")
	return nil
}

func (p *LogProducer) Close(ctx context.Context) error {
	_ = ctx
	p.logger.Info().Msg("event producer closed")
	return nil
}
