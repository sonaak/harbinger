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
		t.Errorf("expect length to be 1 (actual: %d)", rng.Len())
	}
}


func TestSubscriptionRing_AddNext(t *testing.T) {
	subscriptions := []*subscription {
		&subscription{
			id: 1,
		},
		&subscription{
			id: 5,
		},
		&subscription{
			id: 8,
		},
		&subscription{
			id: 15,
		},
		&subscription{
			id: 0,
		},
	}

	rng := NewSubscriptionRing(subscriptions[0])
	for _, subs := range subscriptions[1:] {
		rng.Add(subs)
	}

	if rng.Len() != 5 {
		t.Errorf("expect there to be 5 elements on ring (actual: %d)", rng.Len())
	}

	for i := 0; i < len(subscriptions); i++ {
		headId := rng.head.Value.(*subscription).id
		if headId != 1 {
			t.Errorf("expect the head to be unchanged (actual id: %d)", headId)
		}

		expectId := subscriptions[i].id
		actualId := rng.current.Value.(*subscription).id
		if expectId != actualId {
			t.Errorf("expect and actual subscription ids don't match: (expect: %d, actual: %d)",
				expectId, actualId)
		}
		rng.Next()
	}
}