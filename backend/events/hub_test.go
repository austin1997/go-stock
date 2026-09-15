package events

import (
	"sync"
	"testing"
)

func TestSendToDeliversToTarget(t *testing.T) {
	h := NewHub()
	c := h.Subscribe("c1")

	h.SendTo("c1", Event{Name: "ping", Data: 1})

	select {
	case ev := <-c.Events():
		if ev.Name != "ping" || ev.Data != 1 {
			t.Fatalf("unexpected event: %+v", ev)
		}
	default:
		t.Fatal("expected event on subscribed client")
	}
}

func TestSendToUnknownIDDrops(t *testing.T) {
	h := NewHub()
	c := h.Subscribe("c1")

	h.SendTo("missing", Event{Name: "agent-message", Data: "secret"})

	select {
	case ev := <-c.Events():
		t.Fatalf("expected directed event to be dropped, got %+v", ev)
	default:
	}
}

func TestSendToAfterUnsubscribeDoesNotBroadcast(t *testing.T) {
	h := NewHub()
	target := h.Subscribe("caller")
	other := h.Subscribe("other")
	h.Unsubscribe(target)

	h.SendTo("caller", Event{Name: "newChatStream", Data: "private"})

	select {
	case ev := <-other.Events():
		t.Fatalf("stale directed event leaked to other client: %+v", ev)
	default:
	}
}

func TestSendToConcurrentReconnectDoesNotPanic(t *testing.T) {
	h := NewHub()
	const id = "client-1"

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 2000; i++ {
			c := h.Subscribe(id)
			go func(c *Client) {
				for range c.Events() {
				}
			}(c)
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 20000; i++ {
			h.SendTo(id, Event{Name: "tick", Data: i})
		}
	}()

	wg.Wait()
}

func TestBroadcastUserIsolatesClients(t *testing.T) {
	h := NewHub()
	a := h.SubscribeUser("a", 1)
	b := h.SubscribeUser("b", 2)

	h.BroadcastUser(1, Event{Name: "stock_price", Data: "secret-a"})

	select {
	case ev := <-a.Events():
		if ev.Data != "secret-a" {
			t.Fatalf("user A got %+v", ev)
		}
	default:
		t.Fatal("user A should receive")
	}
	select {
	case ev := <-b.Events():
		t.Fatalf("user B leaked: %+v", ev)
	default:
	}
}
