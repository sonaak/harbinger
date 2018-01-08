package harbinger

import "container/ring"

type SubscriptionRing struct {
	current *ring.Ring
	head *ring.Ring
}


func NewSubscriptionRing(sub *subscription) *SubscriptionRing {
	rng := ring.New(1)
	rng.Value = sub
	return &SubscriptionRing {
		current: rng,
		head: rng,
	}
}


func (rng *SubscriptionRing) Do(f func(*subscription)) {
	rng.current.Do(func(i interface{}){
		switch s := i.(type) {
		case *subscription:
			f(s)
		}
	})
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
	newNode.Link(rng.head)
}


// Remove - remove a the first occurrence, if any, of a given subscription and returns
// true. If the node is not removed, the function returns false.
func (rng *SubscriptionRing) Remove(sub *subscription) bool {
	node := rng.head
	for i := 0; i < rng.current.Len(); i++ {
		next := node.Next()
		nextSub, ok := next.Value.(*subscription)
		if ok && nextSub.id == sub.id {
			node.Unlink(1)
			return true
		}
		node = node.Next()
	}

	return false
}


type Hub struct {
	requests chan sigReq

	subscriptions []*subscription
}

type subscription struct {
	Signals chan interface{}
	id uint
}

type sigReqType uint

const (
	sub = sigReqType(0)
	unsub = sigReqType(1)
	signal = sigReqType(2)
	broadcast = sigReqType(3)
)

type sigReq interface {
	Type() sigReqType
}

type subreq struct {
	*subscription
}

type unsubreq subreq

func (req *unsubreq) Type() sigReqType {
	return unsub
}

func (req *subreq) Type() sigReqType {
	return sub
}

type sigreq struct {
	signal interface{}
}

func (req *sigreq) Type() sigReqType {
	return signal
}

type bcreq sigreq

func (req *bcreq) Type() sigReqType {
	return broadcast
}

func NewSignals() *Hub {
	signals := &Hub{
		requests: make(chan sigReq),
		subscriptions: make([]*subscription, 0),
	}

	go signals.listenToReqs()
	return signals
}

func (hub *Hub) subscribe(sub *subscription) {
	sub.id = uint(len(hub.subscriptions))
	hub.subscriptions = append(hub.subscriptions, sub)
}

func (hub *Hub) unsubscribe(sub *subscription) {
	subscriptions := make([]*subscription, 0, 0)
	for _, sub := range hub.subscriptions {
		if sub.id != sub.id {
			subscriptions = append(subscriptions, sub)
		}
	}

	hub.subscriptions = subscriptions
}

func (hub *Hub) signal(i interface{}, sub *subscription) {

}

func (hub *Hub) listenToReqs() {
	// listen
	for req := range hub.requests {
		switch v := req.(type) {
		case *subreq:
			hub.subscribe(v.subscription)

		case *unsubreq:
			hub.unsubscribe(v.subscription)

		case *sigreq:

		case *bcreq:
		}
	}
}


func (hub *Hub) Subscribe() *subscription {
	sub := &subscription{
		Signals: make(chan interface{}),
	}

	// register channel
	hub.requests <- &subreq {
		subscription: sub,
	}

	return sub
}

func (hub *Hub) Unsubscribe(subscription *subscription) {
	hub.requests <- &unsubreq {
		subscription: subscription,
	}
}


func (hub *Hub) Signal(i interface{}) {

}

func (hub *Hub) Broadcast(i interface{}) {

}