package harbinger

import (
	"github.com/pkg/errors"
	"testing"
	"time"
	"sync"
)

func checkAddOneOp(op Operation) (bool, error) {
	switch v := op.(type) {
	case *addOneOperation:
		if v.Output != v.Input+1 {
			return false, errors.Errorf("expects output to be %d; actual: %d",
				v.Input+1, v.Output,
			)
		}

	default:
		return false, errors.New("expects all output type to be addOneOperation")
	}

	return true, nil
}

func TestWorkerPool_Execute(t *testing.T) {
	pool := setupHappyPath()
	pool.Start()

	ops := []Operation{
		newAddOneOperation(1, 1*time.Millisecond),
		newAddOneOperation(6, 3*time.Millisecond),
	}
	defer pool.Shutdown()

	resp, err := pool.Execute(ops)
	if err != nil {
		t.Errorf("should not error out: %v", err)
	}

	testWithTimeout(
		t,
		func(t *testing.T) {
			for op := range resp {
				valid, err := checkAddOneOp(op)
				if !valid {
					t.Error(err.Error())
				}
			}
		},
		1*time.Second,
	)
}

func TestWorkerPool_ExecuteWithoutStart(t *testing.T) {
	pool := setupHappyPath()
	ops := []Operation{
		newAddOneOperation(1, 1*time.Millisecond),
		newAddOneOperation(6, 3*time.Millisecond),
	}
	defer pool.Shutdown()
	testWithTimeout(
		t,
		func(t *testing.T) {
			resp, err := pool.Execute(ops)
			if err == nil {
				t.Error("expects error when executing against an unstarted pool")
			}

			_, isOpen := <-resp
			if isOpen {
				t.Error("expects output channel to be closed")
			}

		},
		1*time.Second)
}

type addOneRetryWorker struct {
	retryCount uint

	RetryErr error
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
		retryCount:   retryCount,
		RetryErr:     retryErr,
		addOneWorker: addOneWorker{},
	}
}

func TestWorkerPool_ExecuteRetry(t *testing.T) {
	workers := []Worker{
		newAddOneRetryWorker(errors.New("error: something bad happened"), 3),
		newAddOneRetryWorker(errors.New("error: something bad happened"), 4),
	}

	pool := NewPool(workers)
	pool.Start()
	defer pool.Shutdown()

	ops := []Operation{
		newAddOneOperation(1, 1*time.Millisecond),
		newAddOneOperation(6, 3*time.Millisecond),
	}

	testWithTimeout(t, func(t *testing.T) {
		resp, err := pool.Execute(ops)
		if err != nil {
			t.Errorf("expect there not to be any errors: %v", err)
		}

		for range resp {
		}
	}, 2*time.Second)
}

func TestWorkerPool_ExecuteIsParallel(t *testing.T) {
	// setup three workers
	pool := setupHappyPath()

	// start the pool
	pool.Start()

	// and make sure to tear the whole bloody thing down when done
	defer pool.Shutdown()

	// enqueue some operations
	ops := []Operation{
		newAddOneOperation(1, 10*time.Millisecond),
		newAddOneOperation(6, 10*time.Millisecond),
		newAddOneOperation(6, 12*time.Millisecond),
		newAddOneOperation(6, 2*time.Millisecond),
	}

	// make sure the request does not take longer than 20ms (probably
	// can cut that down a bit)
	testWithTimeout(t,
		func(t *testing.T) {
			resp, err := pool.Execute(ops)
			if err != nil {
				t.Errorf("expect there not to be any errors: %v", err)
			}

			for op := range resp {
				valid, err := checkAddOneOp(op)
				if !valid {
					t.Error(err.Error())
				}
			}
		}, 20*time.Millisecond)
}

func TestWorkerPool_ExecuteMultiple(t *testing.T) {
	pool := setupHappyPath()
	pool.Start()
	defer pool.Shutdown()

	ops1 := []Operation{
		newAddOneOperation(1, 10*time.Millisecond),
		newAddOneOperation(6, 10*time.Millisecond),
		newAddOneOperation(6, 12*time.Millisecond),
		newAddOneOperation(6, 2*time.Millisecond),
	}

	ops2 := []Operation{
		newAddOneOperation(1, 20*time.Millisecond),
		newAddOneOperation(6, 30*time.Millisecond),
	}

	testWithTimeout(t,
		func(t *testing.T) {
			resp1, err := pool.Execute(ops1)
			if err != nil {
				t.Errorf("expect there not to be any errors: %v", err)
				return
			}

			resp2, err := pool.Execute(ops2)
			if err != nil {
				t.Errorf("expect there not to be any errors: %v", err)
				return
			}

			for op := range resp1 {
				valid, err := checkAddOneOp(op)
				if !valid {
					t.Error(err.Error())
				}
			}

			for op := range resp2 {
				valid, err := checkAddOneOp(op)
				if !valid {
					t.Error(err.Error())
				}
			}
		}, 50*time.Millisecond)
}


func TestWorkerPool_ExecuteMultipleThenShutdown(t *testing.T) {
	pool := setupHappyPath()
	pool.Start()

	opsCollection := [][]Operation{
		{
			newAddOneOperation(6, 12*time.Millisecond),
			newAddOneOperation(6, 2*time.Millisecond),
		},
		{
			newAddOneOperation(1, 20*time.Millisecond),
			newAddOneOperation(6, 30*time.Millisecond),
		},
		{
			newAddOneOperation(1, 20*time.Millisecond),
			newAddOneOperation(6, 30*time.Millisecond),
		},
	}

	wg := sync.WaitGroup{}
	testWithTimeout(t, func(t *testing.T){
		wg.Add(3)
		for _, ops := range opsCollection {
			go func(ops []Operation) {
				defer wg.Done()
				_, err := pool.Execute(ops)

				if err != nil {
					t.Errorf("expect there not to be any errors: %v", err)
				}
			}(ops)
		}
		wg.Wait()
		pool.Shutdown()
	}, 1 * time.Second)
}
