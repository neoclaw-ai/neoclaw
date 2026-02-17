package runtime

import "context"

// Message is an inbound message delivered by a channel transport.
type Message struct {
	Text string
}

// ResponseWriter sends handler responses back to the active channel transport.
type ResponseWriter interface {
	WriteMessage(ctx context.Context, text string) error
}

// Handler processes inbound messages and writes responses.
type Handler interface {
	HandleMessage(ctx context.Context, w ResponseWriter, msg *Message) error
}

// Listener receives channel input and dispatches it to a Handler.
type Listener interface {
	Listen(ctx context.Context, handler Handler) error
}
