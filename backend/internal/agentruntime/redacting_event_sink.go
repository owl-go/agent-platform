package agentruntime

import "context"

type payloadRedactor interface {
	Bytes([]byte) []byte
}

type redactingEventSink struct {
	redactor payloadRedactor
	next     EventSink
}

// NewRedactingEventSink filters event payloads at the persistence boundary.
func NewRedactingEventSink(redactor payloadRedactor, next EventSink) EventSink {
	return redactingEventSink{redactor: redactor, next: next}
}

func (s redactingEventSink) Publish(ctx context.Context, event Event) error {
	event.Payload = s.redactor.Bytes(event.Payload)
	return s.next.Publish(ctx, event)
}
