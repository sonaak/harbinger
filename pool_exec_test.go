package harbinger

import (
	"testing"
	"time"
	"github.com/pkg/errors"
)


func TestActorPool_Execute(t *testing.T) {
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


func TestActorPool_ExecuteWithoutStart(t *testing.T) {
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
		if worker.retryCount < v.Try && worker.RetryErr != nil {
			return true, worker.RetryErr
		}

		v.Output = v.Input + 1
		return false, nil

	default:
		return false, errors.New("wrong operation type")
	}

	return true, nil
}


func TestWorkerPool_ExecuteRetry(t *testing.T) {

}
