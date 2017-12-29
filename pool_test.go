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

func testWithTimeout(t *testing.T, f func(*testing.T), d time.Duration) {
	timeoutErr := timeout(func(){
		f(t)
	}, d)

	if timeoutErr != nil {
		t.Errorf("expect not to timeout after %s", d.String())
	}
}

func setupHappyPath() *WorkerPool {
	workers := []Worker{
		&addOneWorker{},
		&addOneWorker{},
		&addOneWorker{},
	}
	return NewPool(workers)
}

// Tests that starting the worker pool is going to be OK
func TestWorkerPool_Start(t *testing.T) {
	pool := setupHappyPath()

	testWithTimeout(t, func(t *testing.T) {
		err := pool.Start()
		if err != nil {
			t.Error("should not encounter any error in start")
		}
	}, 1 * time.Second)
}

func parallelTestStart(pool *WorkerPool, wg *sync.WaitGroup, t *testing.T) {
	err := pool.Start()
	if err != nil {
		t.Errorf("should not encounter any error in start")
	}
	wg.Done()
}

// Tests that starting the worker pool is idempotent
func TestWorkerPool_Start_Idempotence(t *testing.T) {
	pool := setupHappyPath()

	wg := sync.WaitGroup{}
	wg.Add(3)
	go parallelTestStart(pool, &wg, t)
	go parallelTestStart(pool, &wg, t)
	go parallelTestStart(pool, &wg, t)

	testWithTimeout(t, func(t *testing.T) {
		wg.Wait()
	}, 1 * time.Second)

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

func TestWorkerPool_Startup_InitFail(t *testing.T) {
	workers := []Worker{
		&addOneWorker{},
		&initFailWorker{&addOneWorker{}},
		&addOneWorker{},
	}
	pool := NewPool(workers)
	testWithTimeout(t, func(t *testing.T) {
		err := pool.Start()
		if err == nil {
			t.Errorf("start with bad worker should result in failure")
		}

		// this channel should be closed
		for range pool.operationChan {
		}
	}, 2 * time.Second)
}


func Test_startupReq_Type(t *testing.T) {
	req := startupReq{}
	if req.Type() != startup {
		t.Error("expect startupReq instance to have type startup")
	}
}


func Test_shutdownReq_Type(t *testing.T) {
	req := shutdownReq(true)
	if req.Type() != shutdown {
		t.Error("expect shutdownReq instance to have type shutdown")
	}
}


func Test_executeReq_Type(t *testing.T) {
	req := executeReq{}
	if req.Type() != execute {
		t.Error("expect executeReq instance to have type execute")
	}
}


func Test_doSingleReq_Type(t *testing.T) {
	req := doSingleReq{}
	if req.Type() != dosingle {
		t.Error("expect doSingleReq instance to have type dosingle")
	}
}


func Test_wrapStreamReq_Type(t *testing.T) {
	req := wrapStreamReq{}
	if req.Type() != wrap {
		t.Error("expect wrapStreamReq instance to have type wrap")
	}
}


func Test_redriveReq_Type(t *testing.T) {
	req := redriveReq{}
	if req.Type() != redrive {
		t.Error("expect redriveReq instance to have type redrive")
	}
}
