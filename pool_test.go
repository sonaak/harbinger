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
	initCount  uint
	id         uint
	handledErr error
	handledOp  Operation
}

func (worker *addOneWorker) Init() error {
	worker.initCount++
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

func setupHappyPath() *WorkerPool {
	workers := []Worker{
		&addOneWorker{},
		&addOneWorker{},
	}
	return NewPool(workers)
}

// Tests that starting the worker pool is going to be OK
func TestActorPool_Start(t *testing.T) {
	pool := setupHappyPath()

	timeoutDuration := 1 * time.Second
	timeoutErr := timeout(func() {
		err := pool.Start()
		if err != nil {
			t.Error("should not encounter any error in start")
		}
	}, timeoutDuration)

	if timeoutErr != nil {
		t.Errorf("should not timeout after %s", timeoutDuration.String())
	}
}

func parallelTestStart(pool *WorkerPool, wg *sync.WaitGroup, t *testing.T) {
	err := pool.Start()
	if err != nil {
		t.Errorf("should not encounter any error in start")
	}
	wg.Done()
}

// Tests that starting the worker pool is idempotent
func TestActorPool_Start_Idempotence(t *testing.T) {
	pool := setupHappyPath()

	wg := sync.WaitGroup{}
	wg.Add(3)
	go parallelTestStart(pool, &wg, t)
	go parallelTestStart(pool, &wg, t)
	go parallelTestStart(pool, &wg, t)

	timeoutErr := timeout(func() {
		wg.Wait()
	}, 1*time.Second)

	if timeoutErr != nil {
		t.Errorf("timed out waiting for 2x starts")
	}

	// assert that each of the workers have only been
	// initialised once
	for _, w := range pool.Workers {
		switch worker := w.(type) {
		case *addOneWorker:
			initCount := worker.initCount
			if initCount != 1 {
				t.Errorf("expects init count 1; %d instead", initCount)
			}
		}
	}
}

type initFailWorker struct {
	*addOneWorker
}

func (worker *initFailWorker) Init() error {
	return errors.New("fail to init")
}

func TestActorPool_Startup_InitFail(t *testing.T) {
	workers := []Worker{
		&addOneWorker{},
		&initFailWorker{&addOneWorker{}},
		&addOneWorker{},
	}
	pool := NewPool(workers)
	timeoutDuration := 2 * time.Second
	timeoutErr := timeout(func() {
		err := pool.Start()
		if err == nil {
			t.Errorf("start with bad worker should result in failure")
		}

		// this channel should be closed
		for range pool.operationChan {
		}
	}, timeoutDuration)

	if timeoutErr != nil {
		t.Errorf("should not timeout after %s", timeoutDuration.String())
	}
}


func TestActorPool_Execute(t *testing.T) {
	pool := setupHappyPath()
	pool.Start()

	ops := []Operation {
		newAddOneOperation(1, 1 * time.Millisecond),
	}

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
					t.Error("expects all output type to be")
				}
			}
		},
		1 * time.Second,
	)

	if timeoutErr != nil {
		t.Error("should not timeout after 1s")
	}
}
