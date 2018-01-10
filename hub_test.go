package harbinger

import "testing"

func TestNewHub(t *testing.T) {
	hub := NewHub()

	if hub.subscriptions.Len() != 0 {
		t.Errorf("expect a new hub to have no subscriptions (actual: %d)", hub.subscriptions.Len())
	}

	if hub.lock == nil {
		t.Error("expect hub.lock to not be nil")
	}
}