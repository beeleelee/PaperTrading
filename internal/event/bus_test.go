package event

import (
	"context"
	"testing"
	"time"
)

type testEvent struct {
	value string
	at    time.Time
}

func (e testEvent) EventName() string   { return "test.event" }
func (e testEvent) Timestamp() time.Time { return e.at }

type counterHandler struct {
	count int
}

func (h *counterHandler) Handle(ctx context.Context, event Event) error {
	h.count++
	return nil
}

func TestBus_Publish(t *testing.T) {
	bus := NewBus()
	handler := &counterHandler{}
	bus.Subscribe("test.event", handler.Handle)

	err := bus.Publish(context.Background(), testEvent{value: "hello", at: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if handler.count != 1 {
		t.Fatalf("expected count 1, got %d", handler.count)
	}
}

func TestBus_MultipleHandlers(t *testing.T) {
	bus := NewBus()
	h1 := &counterHandler{}
	h2 := &counterHandler{}
	bus.Subscribe("test.event", h1.Handle)
	bus.Subscribe("test.event", h2.Handle)

	bus.Publish(context.Background(), testEvent{value: "hello", at: time.Now()})
	if h1.count != 1 || h2.count != 1 {
		t.Fatalf("expected both handlers called, got h1=%d h2=%d", h1.count, h2.count)
	}
}

func TestBus_NoHandlerForEvent(t *testing.T) {
	bus := NewBus()
	err := bus.Publish(context.Background(), testEvent{value: "hello", at: time.Now()})
	if err != nil {
		t.Fatal("should not error when no handlers")
	}
}
