package harbinger

import (
	"github.com/pkg/errors"
	"sync"
	"testing"
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

func newAddOneOperation(input int, waitTime time.Duration) *addOneOperation {
	return &addOneOperation{
		Input:        input,
		WaitDuration: waitTime,
		doneOnce:     sync.Once{},
		done:         make(chan interface{}),
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
	id         uint
	handledErr error
	handledOp  Operation
}

func (worker *addOneWorker) Init() error {
	return nil
}

func (worker *addOneWorker) Process(op Operation) (bool, error) {
	switch v := op.(type) {
	case *addOneOperation:
		time.Sleep(v.WaitDuration)
		v.Output = v.Input + 1

	default:
		return false, errors.New("wrong operation type")
	}

	return true, nil
}

func (worker *addOneWorker) HandleError(err error, op Operation) {
	worker.handledErr = err
	worker.handledOp = op
}

func (worker *addOneWorker) Equal(other Worker) bool {
	switch w := other.(type) {
	case *addOneWorker:
		if w.id != worker.id {
			return false
		}
	default:
		return false
	}

	return true
}

func (worker *addOneWorker) Cleanup() {}

func timeout(f func(), d time.Duration) error {

	done := make(chan interface{})
	expire := make(chan interface{})
	go func() {
		time.Sleep(d)
		expire <- true
	}()

	go func() {
		f()
		done <- true
	}()

	select {
	case <-done:
		return nil

	case <-expire:
		return errors.Errorf("timeout after %s", d.String())
	}
}

func TestActorPool_Start(t *testing.T) {
	workers := []Worker {
		&addOneWorker{},
	}
	pool := NewPool(workers)

	timeoutDuration := 1 * time.Second
	timeoutErr := timeout(func(){
		err := pool.Start()
		if err != nil {
			t.Error("should not encounter any error in start")
		}
	}, timeoutDuration)

	if timeoutErr != nil {
		t.Errorf("should not timeout after %s", timeoutDuration.String())
	}
}
