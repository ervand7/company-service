package outbox

import (
	"context"
	"errors"
	"testing"
	"time"

	appcompany "company-service/internal/application/company"
	appmocks "company-service/internal/application/company/mocks"
	outboxmocks "company-service/internal/infrastructure/outbox/mocks"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
)

func TestPublisherMarksPublishedEvents(t *testing.T) {
	ctx := context.Background()
	eventID := uuid.New()
	event := appcompany.Event{ID: eventID, Type: appcompany.EventCompanyCreated}
	outboxEvent := appcompany.OutboxEvent{ID: eventID, Event: event}
	repo := outboxmocks.NewRepository(t)
	tx := outboxmocks.NewTransactionRunner(t)
	producer := appmocks.NewEventProducer(t)
	publisher := NewPublisher(repo, tx, producer, time.Second, 5, zerolog.Nop())

	tx.On("WithinTransaction", ctx, mock.AnythingOfType("func(context.Context) error")).
		Return(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		}).
		Once()
	repo.On("FetchPending", ctx, uint64(5)).Return([]appcompany.OutboxEvent{outboxEvent}, nil).Once()
	producer.On("Publish", ctx, event).Return(nil).Once()
	repo.On("MarkPublished", ctx, eventID).Return(nil).Once()

	if err := publisher.ProcessBatch(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestPublisherMarksFailedEvents(t *testing.T) {
	ctx := context.Background()
	publishErr := errors.New("publish failed")
	eventID := uuid.New()
	event := appcompany.Event{ID: eventID, Type: appcompany.EventCompanyCreated}
	outboxEvent := appcompany.OutboxEvent{ID: eventID, Event: event}
	repo := outboxmocks.NewRepository(t)
	producer := appmocks.NewEventProducer(t)
	publisher := NewPublisher(repo, nil, producer, time.Second, 1, zerolog.Nop())

	repo.On("FetchPending", ctx, uint64(1)).Return([]appcompany.OutboxEvent{outboxEvent}, nil).Once()
	producer.On("Publish", ctx, event).Return(publishErr).Once()
	repo.On("MarkFailed", ctx, eventID, publishErr).Return(nil).Once()

	if err := publisher.ProcessBatch(ctx); err != nil {
		t.Fatal(err)
	}
	repo.AssertNotCalled(t, "MarkPublished", mock.Anything, mock.Anything)
}

func TestPublisherReturnsMarkFailedErrors(t *testing.T) {
	ctx := context.Background()
	publishErr := errors.New("publish failed")
	markErr := errors.New("mark failed")
	eventID := uuid.New()
	event := appcompany.Event{Type: appcompany.EventCompanyCreated}
	outboxEvent := appcompany.OutboxEvent{ID: eventID, Event: event}
	repo := outboxmocks.NewRepository(t)
	producer := appmocks.NewEventProducer(t)
	publisher := NewPublisher(repo, nil, producer, time.Second, 1, zerolog.Nop())

	repo.On("FetchPending", ctx, uint64(1)).Return([]appcompany.OutboxEvent{outboxEvent}, nil).Once()
	producer.On("Publish", ctx, event).Return(publishErr).Once()
	repo.On("MarkFailed", ctx, eventID, publishErr).Return(markErr).Once()

	err := publisher.ProcessBatch(ctx)
	if !errors.Is(err, markErr) {
		t.Fatalf("expected mark failed error, got %v", err)
	}
}
