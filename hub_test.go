package harbinger

import (
	"reflect"
	"testing"
	"time"
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

func TestHub_Subscribe(t *testing.T) {
	hub := NewHub()

	sub := hub.Subscribe()
	if hub.subscriptions.Len() != 1 {
		t.Errorf("expect there to be 1 subscription (actual: %d)", hub.subscriptions.Len())
	}

	testWithTimeout(t, func(t *testing.T) {
		hub.Signal(int(1))

		s := <-sub.Signals
		switch v := s.(type) {
		case int:
			if v != 1 {
				t.Errorf("expect the signal to be int 1 (actual: %d)", v)
			}

		default:
			t.Errorf("expect type to be int (actual: %s)", reflect.TypeOf(s).Name())
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

	testWithTimeout(t, func(t *testing.T){
		hub.Broadcast(1)

		s := <-sub1.Signals
		switch v := s.(type) {
		case int:
			if v != 1 {
				t.Errorf("expect the signal to be int 1 (actual: %d)", v)
			}

		default:
			t.Errorf("expect type to be int (actual: %s)", reflect.TypeOf(s).Name())
		}

		s = <-sub2.Signals
		switch v := s.(type) {
		case int:
			if v != 1 {
				t.Errorf("expect the signal to be int 1 (actual: %d)", v)
			}

		default:
			t.Errorf("expect type to be int (actual: %s)", reflect.TypeOf(s).Name())
		}

	}, 1*time.Second)
}
