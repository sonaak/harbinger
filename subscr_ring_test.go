package harbinger

import (
	"sort"
	"testing"
)

func TestNewSubscriptionRing(t *testing.T) {
	s := &subscription{
		id:      2,
		Signals: make(chan interface{}),
	}

	rng := NewSubscriptionRing()
	if rng.Len() != 0 {
		t.Errorf("expect rng to have length 0 (actual: %d)", rng.Len())
	}

	rng.Add(s)

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
	subscriptions := []*subscription{
		{id: 1},
		{id: 5},
		{id: 8},
		{id: 15},
		{id: 0},
	}

	rng := NewSubscriptionRing()
	for _, subs := range subscriptions {
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

func TestSubscriptionRing_Remove(t *testing.T) {
	subscriptions := []*subscription{
		{id: 1},
		{id: 5},
		{id: 8},
		{id: 15},
		{id: 0},
	}

	rng := NewSubscriptionRing()
	for _, subs := range subscriptions {
		rng.Add(subs)
	}
	if !rng.Remove(subscriptions[2]) {
		t.Error("expect removal of an added node to result in removal")
	}

	if rng.Len() != 4 {
		t.Errorf("expect length after removal to be 4")
	}

	for i := 0; i < len(subscriptions); i++ {
		if i != 2 {
			expectId := subscriptions[i].id
			actualId := rng.current.Value.(*subscription).id
			if expectId != actualId {
				t.Errorf("expect and actual subscription ids don't match: (expect: %d, actual: %d)",
					expectId, actualId)
			}
			rng.Next()
		}
	}

	if rng.Remove(subscriptions[2]) {
		t.Error("expect removal of a removed/non-existent node to result in no-op")
	}

	if rng.Len() != 4 {
		t.Errorf("expect a no-op removal to result in unchanged length")
	}
}

func TestSubscriptionRing_RemoveHead(t *testing.T) {
	subscriptions := []*subscription{
		{id: 1},
		{id: 5},
		{id: 8},
		{id: 15},
		{id: 0},
	}

	rng := NewSubscriptionRing()
	for _, subs := range subscriptions {
		rng.Add(subs)
	}

	if !rng.Remove(subscriptions[0]) {
		t.Error("expect removal of head to succeed")
	}

	if rng.Len() != 4 {
		t.Errorf("expect removal of head to change number of elts in ring (expected: 4, actual: %d)", rng.Len())
	}

	headId := rng.head.Value.(*subscription).id
	if headId != 5 {
		t.Errorf("expect removal of head/current to change to next (expected: 5, actual: %d)", headId)
	}
}

func TestSubscriptionRing_Do(t *testing.T) {
	subscriptions := []*subscription{
		{id: 1},
		{id: 5},
		{id: 8},
		{id: 15},
		{id: 0},
	}

	rng := NewSubscriptionRing()
	for _, subs := range subscriptions {
		rng.Add(subs)
	}

	processedIds := []int{}
	rng.Do(func(s *subscription) {
		processedIds = append(processedIds, int(s.id))
	})

	sort.Ints(processedIds)
	expectedList := []int{0, 1, 5, 8, 15}
	for i, e := range expectedList {
		if processedIds[i] != e {
			t.Errorf("expect subscription %d to be processed", e)
			return
		}
	}
}

func TestSubscriptionRing_Current(t *testing.T) {
	subscriptions := []*subscription{
		{id: 1},
		{id: 5},
		{id: 8},
		{id: 15},
		{id: 0},
	}

	rng := NewSubscriptionRing()
	for _, subs := range subscriptions {
		rng.Add(subs)
	}

	rng.Next()
	rng.Next()
	if rng.Current().id != 8 {
		t.Errorf("expect Current() to have id 8 (actual: %d)", rng.Current().id)
	}
}
