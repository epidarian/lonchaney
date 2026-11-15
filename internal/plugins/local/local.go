package local

import (
	"sync"

	"lonchan/internal/caps"
	"lonchan/internal/model"
)

// Event is a message published inside a room.
type Event struct {
	Room    string
	Message model.LogMessage
}

// Broker delivers local room messages to every subscriber.
type Broker struct {
	mu   sync.Mutex
	next int
	subs map[string]map[int]chan Event
}

func New() *Broker {
	return &Broker{subs: map[string]map[int]chan Event{}}
}

func (b *Broker) Capabilities() caps.Caps { return caps.Full() }

// Subscribe returns a stream of events for a room.
func (b *Broker) Subscribe(room string) (<-chan Event, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.next++
	id := b.next
	ch := make(chan Event, 16)
	if b.subs[room] == nil {
		b.subs[room] = map[int]chan Event{}
	}
	b.subs[room][id] = ch
	return ch, func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		delete(b.subs[room], id)
		close(ch)
	}
}

// Publish sends a message to every subscriber of the room.
func (b *Broker) Publish(room string, msg model.LogMessage) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, ch := range b.subs[room] {
		select {
		case ch <- Event{Room: room, Message: msg}:
		default:
		}
	}
}
