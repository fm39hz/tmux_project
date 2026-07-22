package event

import "context"

type Event string

const (
	ShapeSaved Event = "shape.saved"
	FreezeDone Event = "freeze.done"
)

type Handler func(ctx context.Context, args ...any)

type Bus struct {
	handlers map[Event][]Handler
}

func New() *Bus { return &Bus{handlers: make(map[Event][]Handler)} }

func (b *Bus) On(event Event, fn Handler) {
	b.handlers[event] = append(b.handlers[event], fn)
}

func (b *Bus) Emit(ctx context.Context, event Event, args ...any) {
	for _, fn := range b.handlers[event] {
		fn(ctx, args...)
	}
}
