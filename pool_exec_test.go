package harbinger

import (
	"testing"
	"time"
	"github.com/pkg/errors"
)


func TestWorkerPool_Execute(t *testing.T) {
	pool := setupHappyPath()
	pool.Start()

	ops := []Operation {
		newAddOneOperation(1, 1 * time.Millisecond),
		newAddOneOperation(6, 3 * time.Millisecond),
	}
	defer pool.Shutdown()

	resp, err := pool.Execute(ops)
	if err != nil {
		t.Errorf("should not error out: %v", err)
	}

	timeoutErr := timeout(
		func(){
			for op := range resp {
				switch v := op.(type) {
				case *addOneOperation:
					if v.Output != v.Input + 1 {
						t.Errorf("expects output to be %d; actual: %d",
							v.Input + 1, v.Output,
						)
					}

				default:
					t.Error("expects all output type to be addOneOperation")
				}
			}
		},
		1 * time.Second,
	)

	if timeoutErr != nil {
		t.Error("should not timeout after 1s")
	}
}


func TestWorkerPool_ExecuteWithoutStart(t *testing.T) {
	pool := setupHappyPath()
	ops := []Operation {
		newAddOneOperation(1, 1 * time.Millisecond),
		newAddOneOperation(6, 3 * time.Millisecond),
	}
	defer pool.Shutdown()

	timeoutErr := timeout(func(){
		resp, err := pool.Execute(ops)
		if err == nil {
			t.Error("expects error when executing against an unstarted pool")
		}

		_, isOpen := <-resp
		if isOpen {
			t.Error("expects output channel to be closed")
		}

	}, 1 * time.Second)

	if timeoutErr != nil {
		t.Error("should not timeout after 1s")
	}

}


type addOneRetryWorker struct {
	retryCount uint

	RetryErr   error
	addOneWorker
}


func (worker *addOneRetryWorker) Process(op Operation) (retry bool, err error) {
	switch v := op.(type) {
	case *addOneOperation:
		if worker.retryCount > v.Try && worker.RetryErr != nil {
			return true, worker.RetryErr
		}

		return worker.addOneWorker.Process(op)

	default:
		return false, errors.New("wrong operation type")
	}
}


func newAddOneRetryWorker(retryErr error, retryCount uint) *addOneRetryWorker {
	return &addOneRetryWorker{
		retryCount: retryCount,
		RetryErr: retryErr,
		addOneWorker: addOneWorker{},
	}
}


func TestWorkerPool_ExecuteRetry(t *testing.T) {
	workers := []Worker {
		newAddOneRetryWorker(errors.New("error: something bad happened"), 3),
		newAddOneRetryWorker(errors.New("error: something bad happened"), 4),
	}

	pool := NewPool(workers)
	pool.Start()
	defer pool.Shutdown()

	ops := []Operation {
		newAddOneOperation(1, 1 * time.Millisecond),
		newAddOneOperation(6, 3 * time.Millisecond),
	}

	timeoutErr := timeout(func() {
		resp, err := pool.Execute(ops)
		if err != nil {
			t.Errorf("expect there not to be any errors: %v", err)
		}

		for range resp {}
	}, 2 * time.Second)

	if timeoutErr != nil {
		t.Error("should not time out after 2s")
	}
}


func TestWorkerPool_ExecuteIsParallel(t *testing.T) {
	pool := setupHappyPath()
	pool.Start()
	defer pool.Shutdown()

	timeoutErr := tmeout(func(){

	})
}
