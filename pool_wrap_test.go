package harbinger

import (
	"github.com/pkg/errors"
	"testing"
	"time"
)

func TestWorkerPool_Wrap(t *testing.T) {
	pool := setupHappyPath()
	pool.Start()
	defer pool.Shutdown()

	inputStream := make(chan Operation)
	ops := []Operation{
		newAddOneOperation(1, 10*time.Millisecond),
		newAddOneOperation(6, 10*time.Millisecond),
		newAddOneOperation(6, 12*time.Millisecond),
		newAddOneOperation(6, 2*time.Millisecond),
	}

	go func(ops []Operation) {
		for _, op := range ops {
			inputStream <- op
		}
		close(inputStream)
	}(ops)

	testWithTimeout(t, func(t *testing.T) {
		outputStream, err := pool.Wrap(inputStream)
		if err != nil {
			t.Errorf("expect there not to be any errors: %v", err)
		}
		for op := range outputStream {
			valid, err := checkAddOneOp(op)
			if !valid {
				t.Error(err.Error())
			}
		}
	}, 1*time.Second)
}

func TestWorkerPool_WrapShutdown(t *testing.T) {
	pool := setupHappyPath()
	pool.Start()

	inputStream := make(chan Operation)
	ops := []Operation{
		newAddOneOperation(1, 10*time.Millisecond),
		newAddOneOperation(6, 10*time.Millisecond),
		newAddOneOperation(6, 12*time.Millisecond),
		newAddOneOperation(6, 2*time.Millisecond),
	}

	go func(ops []Operation) {
		for _, op := range ops {
			inputStream <- op
		}
		close(inputStream)
	}(ops)

	testWithTimeout(t, func(t *testing.T) {
		outputStream, err := pool.Wrap(inputStream)
		if err != nil {
			t.Errorf("expect there not to be any errors: %v", err)
			return
		}

		pool.Shutdown()
		for op := range outputStream {
			valid, err := checkAddOneOp(op)
			if !valid {
				t.Error(err.Error())
			}
		}
	}, 1*time.Second)
}

func TestWorkerPool_WrapWithRetry(t *testing.T) {
	//t.Skip("This fails pretty regularly: see #14")

	workers := []Worker{
		newAddOneRetryWorker(errors.New("error: something bad happened"), 3),
		newAddOneRetryWorker(errors.New("error: something bad happened"), 4),
	}

	pool := NewPool(workers)
	pool.Start()
	// TODO: deadlock with defer pool.Shutdown()
	defer pool.Shutdown()

	inputStream := make(chan Operation)
	ops := []Operation{
		newAddOneOperation(1, 10*time.Millisecond),
		newAddOneOperation(6, 10*time.Millisecond),
		newAddOneOperation(6, 12*time.Millisecond),
		newAddOneOperation(6, 2*time.Millisecond),
	}

	go func(ops []Operation) {
		for _, op := range ops {
			inputStream <- op
		}
		close(inputStream)
	}(ops)

	testWithTimeout(t, func(t *testing.T) {
		outputStream, err := pool.Wrap(inputStream)
		if err != nil {
			t.Errorf("expect there not to be any errors: %v", err)
			return
		}
		pool.Shutdown()

		for op := range outputStream {
			valid, err := checkAddOneOp(op)
			if !valid {
				t.Error(err.Error())
			}
		}
	}, 2*time.Second)
}

func TestWorkerPool_WrapBeforeStart(t *testing.T) {
	pool := setupHappyPath()
	inputStream := make(chan Operation)
	ops := []Operation{
		newAddOneOperation(1, 10*time.Millisecond),
		newAddOneOperation(6, 10*time.Millisecond),
		newAddOneOperation(6, 12*time.Millisecond),
		newAddOneOperation(6, 2*time.Millisecond),
	}

	go func(ops []Operation) {
		for _, op := range ops {
			inputStream <- op
		}
	}(ops)

	outputStream, err := pool.Wrap(inputStream)
	if err == nil {
		t.Error("expects err from wrapping without starting")
	}

	testWithTimeout(t, func(t *testing.T) {
		for range outputStream {
			t.Error("expect output to be empty")
		}
	}, 3*time.Second)
}
