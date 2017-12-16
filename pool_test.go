package harbinger

import (
	"sync"
	"time"
)

type addOneOperation struct {
	Input        int
	WaitDuration time.Duration
	Output       int
	Try          uint

	doneOnce sync.Once
	done     chan interface{}
}

func newAddOneOperation(input int) *addOneOperation {
	return &addOneOperation{
		Input:    input,
		doneOnce: sync.Once{},
		done:     make(chan interface{}),
	}
}

func (op *addOneOperation) IncrementTry() {
	op.Try++
}

func (op *addOneOperation) Wait() {
	for range op.done {
	}
}

func (op *addOneOperation) Done() {
	op.doneOnce.Do(func() {
		close(op.done)
	})
}

type addOneWorker struct {
}
