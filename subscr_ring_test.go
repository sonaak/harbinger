package harbinger

import "testing"



func TestNewSubscriptionRing(t *testing.T) {
	s := &subscription {
		id: 2,
		Signals: make(chan interface{}),
	}

	rng := NewSubscriptionRing(s)

	rngHead := rng.head
	rngCurrent := rng.current

	if rngHead.Value.(*subscription).id != 2 {
		t.Error("expect head to be a subscription with id 2")
	}

	if rngCurrent.Value.(*subscription).id != 2 {
		t.Error("expect current to be a subscription with id 2")
	}

	if rng.Len() != 1 {
		t.Error("expect length to be 1 (actual: %d)", rng.Len())
	}
}