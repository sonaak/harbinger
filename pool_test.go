package harbinger

// Author: Knight Fu
// Date: whenever
//
// Preamble: FYI, this is a really shitty way to add 1 to a bunch
// of numbers. Don't do this, even if you're just trying to do this
// in a pool of threads, this may still be a really shitty thing to
// do.

import (
	"github.com/pkg/errors"
	"github.com/satori/go.uuid"
	"sync"
	"testing"
	"time"
	"fmt"
)

type AddOneOp struct {
	Input  int
	Output int

	RetryCount int
	done       chan interface{}
	doneOnce   *sync.Once
}

func NewAddOneOp(input int) *AddOneOp {
	return &AddOneOp{
		Input:      input,
		done:       make(chan interface{}),
		RetryCount: 0,
		doneOnce:   &sync.Once{},
	}
}

func (op *AddOneOp) IncrementTry() {
	op.RetryCount += 1
}
func (op *AddOneOp) Wait() {
	for range op.done {
	}
}

func (op *AddOneOp) Done() {
	op.doneOnce.Do(func() {
		close(op.done)
	})
}

type AddOneActor struct {
	Id       uuid.UUID
	WaitTime time.Duration

	initError error

	initialised bool
}

func NewAddOneActor(waitTime time.Duration, initErr error) *AddOneActor {
	return &AddOneActor{
		Id:       uuid.NewV1(),
		WaitTime: waitTime,

		initError: initErr,
	}
}

func (actor *AddOneActor) Init() error {
	if actor.initError != nil {
		return actor.initError
	}

	actor.initialised = true
	return nil
}

func (actor *AddOneActor) Process(op Operation) (bool, error) {
	addOneOp, ok := op.(*AddOneOp)
	if !ok {
		return false, errors.New("The operation cannot be processed by this actor.")
	}

	time.Sleep(actor.WaitTime)
	addOneOp.Output = addOneOp.Input + 1
	return false, nil
}

func (actor *AddOneActor) HandleError(err error, op Operation) {}
func (actor *AddOneActor) Equal(worker Worker) bool {
	addOneActor, ok := worker.(*AddOneActor)
	if !ok {
		return false
	}

	return addOneActor.Id == actor.Id
}

func (actor *AddOneActor) Cleanup() {}

func sameWorker(a Worker, b Worker, testIndex int) (bool, string) {
	_, ok := a.(*AddOneActor)
	if !ok {
		return false, fmt.Sprintf("TC %d: Expected worker type to be AddOneActor. Didn't get that.", testIndex)
	}

	if !a.Equal(b) {
		return false, fmt.Sprintf("TC %d: Expected worker to be the same as the ones that went in.", testIndex)
	}

	return true, ""
}

func sameWorkerGroup(workers []Worker, expectedWorker []Worker, testIndex int) (bool, string) {
	for i, worker := range workers {
		ok, errMsg := sameWorker(worker, expectedWorker[i], testIndex)
		if !ok {
			return false, errMsg
		}
	}

	return true, ""
}


type TestNewPoolTestCase struct {
	Workers []Worker
}

func test(testIndex int, testCase TestNewPoolTestCase, t *testing.T) {
	pool := NewPool(testCase.Workers)

	// ensure that the worker pool is exactly the one
	// that we created
	if len(pool.Workers) != len(testCase.Workers) {
		t.Errorf("TC %d: Expected worker count to be 3. Got %d.",
			testIndex, len(pool.Workers))
	}

	ok, msg := sameWorkerGroup(pool.Workers, testCase.Workers, testIndex)
	if !ok {
		t.Error(msg)
		return
	}

	go func() {
		<-pool.requestChan
	}()

	pool.requestChan <- &AddOneOp{
		Input: 1,
	}
	pool.Shutdown()
}

func TestNewPool(t *testing.T) {

	testCases := []TestNewPoolTestCase {
		{
			Workers: []Worker{},
		},
		{
			Workers: []Worker{
				NewAddOneActor(1*time.Millisecond, nil),
				NewAddOneActor(3*time.Millisecond, nil),
				NewAddOneActor(10*time.Second, nil),
			},
		},
	}

	var testCase int
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("TC %d: we have encountered an unexpected error: %v", testCase, r)
		}
	}()

	for i, c := range testCases {
		test(i, c, t)
	}

}

func TestActorPool_Start(t *testing.T) {
	testDone := make(chan interface{})
	timeout := make(chan interface{})
	go func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Error: an unexpected error occurred: %v", r)
				testDone <- true
			}
		}()

		pool := NewPool([]Worker{
			NewAddOneActor(1*time.Millisecond, nil),
		})

		pool.Start()
		time.Sleep(1 * time.Millisecond)
		pool.Shutdown()

		testDone <- true

	}()

	go func() {
		// timeout
		time.Sleep(1 * time.Second)
		timeout <- true
	}()

	select {
	case <-testDone:
	case <-timeout:
		t.Error("Expected the Start operation to be non-blocking.")
	}
}

func TestActorPool_Start_InitFail(t *testing.T) {
	testDone := make(chan interface{})
	timeout := make(chan interface{})
	go func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Error: an unexpected error occurred: %v", r)
				testDone <- true
			}
		}()

		pool := NewPool([]Worker{
			NewAddOneActor(1*time.Millisecond, errors.New("Bad news.")),
		})

		err := pool.Start()
		if err == nil {
			t.Error("Expected there to be an error in startup.")
		}
		testDone <- true
	}()

	go func() {
		time.Sleep(1 * time.Second)
		timeout <- true
	}()

	select {
	case <-testDone:
	case <-timeout:
		t.Error("Expected the Start operation to be non-blocking.")
	}
}

func TestActorPool_Execute(t *testing.T) {
	testDone := make(chan interface{})
	timeout := make(chan interface{})

	go func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Error: an unexpected error occurred: %v", r)
				testDone <- true
			}
		}()

		pool := NewPool([]Worker{
			NewAddOneActor(1*time.Millisecond, nil),
		})

		err := pool.Start()
		if err != nil {
			t.Error("Expected there to be no error in startup.")
		}

		results := pool.Execute([]Operation{
			NewAddOneOp(2),
			NewAddOneOp(4),
		})

		// test results are actually computed
		for result := range results {
			addOneResult, ok := result.(*AddOneOp)
			if !ok {
				t.Errorf("The type of op somehow got corrupted. Expected AddOneOp.")
			}

			if addOneResult.Output != addOneResult.Input+1 {
				t.Errorf("Expected AddOneOp to have added one to %d (input). Got %d instead.",
					addOneResult.Input, addOneResult.Output,
				)
			}
		}
		testDone <- true

	}()

	go func() {
		time.Sleep(1 * time.Second)
		timeout <- true
	}()

	select {
	case <-testDone:

		// test that the channel from the test results will actually be closed
	case <-timeout:
		t.Error("Expected the Start operation to be non-blocking.")
	}
}

type FailingAddOneWorker struct {
	RetryCount int
	Error      error

	handledErr error
	handledOp  Operation
}

func NewFailingAddOneWorker(retryCount int, err error) *FailingAddOneWorker {
	return &FailingAddOneWorker{
		RetryCount: retryCount,
		Error:      err,
	}
}

func (actor *FailingAddOneWorker) Process(op Operation) (bool, error) {
	addOneOp, _ := op.(*AddOneOp)
	if addOneOp.RetryCount < actor.RetryCount {
		return true, actor.Error
	}

	return false, actor.Error
}
func (actor *FailingAddOneWorker) Init() error { return nil }
func (actor *FailingAddOneWorker) HandleError(err error, op Operation) {
	actor.handledErr = err
	actor.handledOp = op
}
func (actor *FailingAddOneWorker) Equal(Worker) bool { return true }
func (actor *FailingAddOneWorker) Cleanup()          {}

type PanickingAddOneWorker struct {
	wait      time.Duration
	cleanedUp bool
}

func NewPanickingAddOneWorker(wait time.Duration) *PanickingAddOneWorker {
	return &PanickingAddOneWorker{
		wait: wait,
	}
}

func (actor *PanickingAddOneWorker) Process(op Operation) (bool, error) {
	time.Sleep(actor.wait)
	panic(errors.New("Something went terribly wrong."))
}

func (actor *PanickingAddOneWorker) Init() error                         { return nil }
func (actor *PanickingAddOneWorker) HandleError(err error, op Operation) {}
func (actor *PanickingAddOneWorker) Equal(Worker) bool                   { return true }
func (actor *PanickingAddOneWorker) Cleanup() {
	actor.cleanedUp = true
}

func TestActorPool_ExecuteWithError(t *testing.T) {
	testDone := make(chan interface{})
	timeout := make(chan interface{})

	go func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Error: an unexpected error occurred: %v", r)
				testDone <- true
			}
		}()

		pool := NewPool([]Worker{
			NewFailingAddOneWorker(
				5,
				errors.New("Failed for some mysterious reasons.")),
		})

		err := pool.Start()
		if err != nil {
			t.Error("Expected there to be no error in startup.")
		}

		results := pool.Execute([]Operation{
			NewAddOneOp(2),
		})

		// test results are actually computed
		for range results {
		}
		testDone <- true
	}()

	go func() {
		time.Sleep(1 * time.Second)
		timeout <- true
	}()

	select {
	case <-testDone:

		// test that the channel from the test results will actually be closed
	case <-timeout:
		t.Error("Timed out: operation seems to be stuck safter 1 second.")
	}
}

func TestActorPool_ExecuteWithPanic(t *testing.T) {
	testDone := make(chan interface{})
	timeout := make(chan interface{})

	go func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Error: an unexpected error occurred: %v", r)
				testDone <- true
			}
		}()

		pool := NewPool([]Worker{
			NewPanickingAddOneWorker(1 * time.Millisecond),
		})

		err := pool.Start()
		if err != nil {
			t.Error("Expected there to be no error in startup.")
		}

		results := pool.Execute([]Operation{
			NewAddOneOp(2),
		})

		go func() {
			time.Sleep(20 * time.Millisecond)
			pool.Shutdown()
		}()

		// test results are actually computed
		for range results {
		}
		testDone <- true

	}()

	go func() {
		time.Sleep(1 * time.Second)
		timeout <- true
	}()

	select {
	case <-testDone:

		// test that the channel from the test results will actually be closed
	case <-timeout:
		t.Error("Timed out: operation seems to be stuck after 1 second.")
	}
}

func TestActorPool_ExecuteWithShutdownBeforePanic(t *testing.T) {
	testDone := make(chan interface{})
	timeout := make(chan interface{})

	go func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Error: an unexpected error occurred: %v", r)
				testDone <- true
			}
		}()

		pool := NewPool([]Worker{
			NewPanickingAddOneWorker(1 * time.Millisecond),
		})

		err := pool.Start()
		if err != nil {
			t.Error("Expected there to be no error in startup.")
		}

		results := pool.Execute([]Operation{
			NewAddOneOp(2),
		})

		go func() {
			pool.Shutdown()
		}()

		// test results are actually computed
		for range results {
		}
		testDone <- true

	}()

	go func() {
		time.Sleep(1 * time.Second)
		timeout <- true
	}()

	select {
	case <-testDone:

		// test that the channel from the test results will actually be closed
	case <-timeout:
		t.Error("Timed out: operation seems to be stuck after 1 second.")
	}
}
