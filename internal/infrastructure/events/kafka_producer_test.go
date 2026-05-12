package events

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	appcompany "company-service/internal/application/company"
	companydomain "company-service/internal/domain/company"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/segmentio/kafka-go"
)

type fakeKafkaWriter struct {
	messages []kafka.Message
	writeErr error
	closeErr error
	closed   bool
}

func (w *fakeKafkaWriter) WriteMessages(_ context.Context, msgs ...kafka.Message) error {
	if w.writeErr != nil {
		return w.writeErr
	}
	w.messages = append(w.messages, msgs...)
	return nil
}

func (w *fakeKafkaWriter) Close() error {
	w.closed = true
	return w.closeErr
}

func TestKafkaProducerPublishesCompanyEvent(t *testing.T) {
	writer := &fakeKafkaWriter{}
	producer := &KafkaProducer{
		writer: writer,
		topic:  "company.events",
		logger: zerolog.Nop(),
	}
	companyID := uuid.New()
	eventID := uuid.New()
	occurredAt := time.Date(2026, 5, 12, 8, 0, 0, 0, time.UTC)

	err := producer.Publish(context.Background(), appcompany.Event{
		ID:         eventID,
		Type:       appcompany.EventCompanyCreated,
		OccurredAt: occurredAt,
		CompanyID:  companyID,
		Company: &companydomain.Company{
			ID:                companyID,
			Name:              "Acme",
			Description:       "description",
			AmountOfEmployees: 10,
			Registered:        true,
			Type:              companydomain.TypeCorporations,
			CreatedAt:         occurredAt,
			UpdatedAt:         occurredAt,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(writer.messages) != 1 {
		t.Fatalf("expected one kafka message, got %d", len(writer.messages))
	}

	message := writer.messages[0]
	if string(message.Key) != companyID.String() {
		t.Fatalf("expected company id key %q, got %q", companyID.String(), message.Key)
	}
	if !message.Time.Equal(occurredAt) {
		t.Fatalf("expected message time %s, got %s", occurredAt, message.Time)
	}

	var payload kafkaEvent
	if err := json.Unmarshal(message.Value, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.ID != eventID.String() || payload.Type != appcompany.EventCompanyCreated || payload.CompanyID != companyID.String() {
		t.Fatalf("unexpected kafka payload: %+v", payload)
	}
	if payload.Company == nil || payload.Company.Name != "Acme" || payload.Company.Type != companydomain.TypeCorporations {
		t.Fatalf("unexpected kafka company payload: %+v", payload.Company)
	}
}

func TestKafkaProducerReturnsWriteErrors(t *testing.T) {
	writeErr := errors.New("write failed")
	producer := &KafkaProducer{
		writer: &fakeKafkaWriter{writeErr: writeErr},
		logger: zerolog.Nop(),
	}

	err := producer.Publish(context.Background(), appcompany.Event{CompanyID: uuid.New()})
	if !errors.Is(err, writeErr) {
		t.Fatalf("expected wrapped write error, got %v", err)
	}
}

func TestKafkaProducerCloseClosesWriter(t *testing.T) {
	writer := &fakeKafkaWriter{}
	producer := &KafkaProducer{writer: writer}

	if err := producer.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !writer.closed {
		t.Fatal("expected writer to be closed")
	}
}

func TestKafkaProducerCloseReturnsErrors(t *testing.T) {
	closeErr := errors.New("close failed")
	producer := &KafkaProducer{writer: &fakeKafkaWriter{closeErr: closeErr}}

	err := producer.Close(context.Background())
	if !errors.Is(err, closeErr) {
		t.Fatalf("expected wrapped close error, got %v", err)
	}
}
