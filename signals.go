package harbinger

import (
	"container/ring"
	"sync"
)

type SubscriptionRing struct {
	current *ring.Ring
	head    *ring.Ring
}

func NewSubscriptionRing() *SubscriptionRing {
	return &SubscriptionRing{}
}

func (rng *SubscriptionRing) Do(f func(*subscription)) {
	rng.current.Do(func(i interface{}) {
		switch s := i.(type) {
		case *subscription:
			f(s)
		}
	})
}

func (rng *SubscriptionRing) Current() *subscription {
	return rng.current.Value.(*subscription)
}

// Len - returns the number of nodes in the ring
// (see container/ring.Ring.Len)
func (rng *SubscriptionRing) Len() int {
	return rng.current.Len()
}

// Next - advances the underlying ring node forward by one
func (rng *SubscriptionRing) Next() {
	rng.current = rng.current.Next()
}

// Add - add a subscription as a ring item to the end
func (rng *SubscriptionRing) Add(sub *subscription) {
	newNode := ring.New(1)
	newNode.Value = sub

	if rng.head != nil {
		newNode.Link(rng.head)
		return
	}

	rng.head = newNode
	rng.current = newNode
}

// Remove - remove a the first occurrence, if any, of a given subscription and returns
// true. If the node is not removed, the function returns false.
func (rng *SubscriptionRing) Remove(sub *subscription) bool {
	node := rng.head
	for i := 0; i < rng.current.Len(); i++ {
		next := node.Next()
		nextSub, ok := next.Value.(*subscription)
		if ok && nextSub.id == sub.id {
			if next == rng.head {
				rng.head = next.Next()
			}
			if next == rng.current {
				rng.current = next.Next()
			}

			node.Unlink(1)

			return true
		}
		node = node.Next()
	}

	return false
}

type Hub struct {
	subscriptions *SubscriptionRing
	lock          *sync.Mutex
}

type subscription struct {
	Signals chan interface{}
	id      uint
}

func NewHub() *Hub {
	return &Hub{
		subscriptions: NewSubscriptionRing(),
		lock:          &sync.Mutex{},
	}
}

func (hub *Hub) Subscribe() *subscription {
	sub := &subscription{
		Signals: make(chan interface{}),
	}

	hub.lock.Lock()
	hub.subscriptions.Add(sub)
	sub.id = uint(hub.subscriptions.Len())
	hub.lock.Unlock()

	return sub
}

func (hub *Hub) Unsubscribe(sub *subscription) {
	hub.lock.Lock()
	defer hub.lock.Unlock()

	hub.subscriptions.Remove(sub)
}

func (hub *Hub) Signal(i interface{}) {
	hub.lock.Lock()
	defer hub.lock.Unlock()
	sub := hub.subscriptions.Current()

	go func() {
		sub.Signals <- i
	}()
	hub.subscriptions.Next()
}

func (hub *Hub) Broadcast(i interface{}) {
	hub.lock.Lock()
	go hub.subscriptions.Do(func(s *subscription) {
		defer hub.lock.Unlock()
		s.Signals <- i
	})
}
