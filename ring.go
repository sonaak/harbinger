package harbinger

import (
	"sync"
)

type asyncReq struct {
	Ref []byte
	done chan interface{}
	doneOnce *sync.Once
	Err error
}

func (req *asyncReq) Wait() {
	for range req.done {}
}


func (req *asyncReq) Done() {
	req.doneOnce.Do(func() {
		close(req.done)
	})
}

func (req *asyncReq) Error() error {
	return req.Err
}


func newReq(p []byte) *asyncReq {
	return &asyncReq {
		Ref: p,
		done: make(chan interface{}),
		doneOnce: &sync.Once{},
		Err: nil,
	}
}


type RingBuffer struct {
	readPtr, writePtr int
	readChan, writeChan chan *asyncReq

	closed bool
	buf []byte
}


func (buf *RingBuffer) listenToReads() {
}


func NewRingBuffer(capacity uint) *RingBuffer {
	return &RingBuffer {
		readPtr: 0,
		writePtr: 0,
		closed: false,

		buf: make([]byte, capacity),
	}
}


func (buffer *RingBuffer) Read(p []byte) (n int, err error) {

	return
}


func (buffer *RingBuffer) Write(p []byte) (n int, err error) {
	return
}

func (buffer *RingBuffer) Close() error {
	return nil
}
