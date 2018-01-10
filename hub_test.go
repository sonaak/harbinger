package harbinger

import (
	"reflect"
	"testing"
	"time"
	"fmt"
)

func TestNewHub(t *testing.T) {
	hub := NewHub()

	if hub.subscriptions.Len() != 0 {
		t.Errorf("expect a new hub to have no subscriptions (actual: %d)", hub.subscriptions.Len())
	}

	if hub.lock == nil {
		t.Error("expect hub.lock to not be nil")
	}
}

func verifySignal(signal chan interface{}, expect int) (valid bool, reason string) {
	valid = true
	s := <- signal

	switch v := s.(type) {
	case int:
		if v != expect {
			valid = false
			reason = fmt.Sprintf("expect the signal to be int %d (actual: %d)", expect, v)
		}

	default:
		valid = false
		reason = fmt.Sprintf("expect type to be int (actual: %s)", reflect.TypeOf(s).Name())
	}

	return
}

func TestHub_Subscribe(t *testing.T) {
	hub := NewHub()

	sub := hub.Subscribe()
	if hub.subscriptions.Len() != 1 {
		t.Errorf("expect there to be 1 subscription (actual: %d)", hub.subscriptions.Len())
	}

	testWithTimeout(t, func(t *testing.T) {
		hub.Signal(int(1))

		valid, reason := verifySignal(sub.Signals, 1)
		if !valid {
			t.Error(reason)
		}
	}, 1*time.Second)
}

func TestHub_Broadcast(t *testing.T) {
	hub := NewHub()

	sub1 := hub.Subscribe()
	sub2 := hub.Subscribe()

	if hub.subscriptions.Len() != 2 {
		t.Errorf("expect there to be 2 subscription (actual: %d)", hub.subscriptions.Len())
	}

	testWithTimeout(t, func(t *testing.T) {
		hub.Broadcast(1)
		if valid, reason := verifySignal(sub1.Signals, 1); !valid {
			t.Error(reason)
		}

		if valid, reason := verifySignal(sub2.Signals, 1); !valid {
			t.Error(reason)
		}
	}, 1*time.Second)
}
