package harbinger

import (
	"sync"
	"github.com/pkg/errors"
)

type Operation interface {
	IncrementTry()

	// Blocks the flow of execution till the operation is done.
	// Must be able to call Wait() from various points in code.
	Wait()

	// Done() signals that the Operation is done. Anyone waiting
	// with Wait() will resume.
	//
	// Unfortunate for you, this has to be idempotent.
	// Minimally, it has to not crash when called consecutively
	Done()
}

type Worker interface {
	Init() error
	Process(op Operation) (bool, error)
	HandleError(error, Operation)
	Equal(Worker) bool
	Cleanup()
}

type reqtype int8

const (
	SHUTDOWN reqtype = 0
	STARTUP reqtype = 1
	EXECUTE reqtype = 2


	STOPPED uint32 = 0
	STARTING uint32 = 1
	RUNNING uint32 = 2
)

type poolreq interface {
	Type() reqtype
}

type AsyncRequest struct {
	Error error

	done chan interface{}
}

func (req *AsyncRequest) Wait() {
	for range req.done {}
}

func (req *AsyncRequest) Done() {
	close(req.done)
}

type StartupRequest struct {
	AsyncRequest
}

func (req *StartupRequest) Type() reqtype {
	return STARTUP
}

type ShutdownRequest bool

func (req ShutdownRequest) Type() reqtype {
	return SHUTDOWN
}

type ExecuteRequest struct {
	AsyncRequest
	Operation []Operation
}


func (req *ExecuteRequest) Type() reqtype {
	return EXECUTE
}


type ActorPool struct {
	Workers []Worker

	operationChan  chan Operation
	reqChan      chan poolreq
	state        uint32
	execWg       *sync.WaitGroup
	closeOnce    *sync.Once
}


func (pool *ActorPool) start() error {
	for _, worker := range pool.Workers {
		if initErr := pool.register(worker); initErr != nil {
			return initErr
		}
	}

	return nil
}


func (pool *ActorPool) enqueue([]Operation) error {
	return nil
}


func (pool *ActorPool) shutdown() {

}


func (pool *ActorPool) dispatch() {
	for req := range pool.reqChan {
		switch v := req.(type){

		case *StartupRequest:
			pool.state = STARTING
			pool.start()
			pool.state = RUNNING

		case *ExecuteRequest:
			pool.execWg.Add(1)
			go func() {
				v.Error = pool.enqueue(v.Operation)
				v.Done()
				pool.execWg.Done()
			}()

		case ShutdownRequest:
			// wait till all the previous executes have completed
			pool.execWg.Wait()

			// then stop everything
			pool.state = STOPPED
			pool.shutdown()
		}
	}
}


func (pool *ActorPool) Start() error {
	req := &StartupRequest{}
	pool.reqChan <- req

	// wait till the request is done
	req.Wait()
	return req.Error
}


func (pool *ActorPool) register(worker Worker) error {
	initErr := worker.Init()
	if initErr != nil {
		return errors.Wrap(initErr, "unable to initialise worker")
	}


	return nil
}


func NewPool(workers []Worker) *ActorPool {
	pool := ActorPool{
		Workers:      workers,
		operationChan:  make(chan Operation),
		reqChan: make(chan poolreq),
		state:   STOPPED,
	}

	go pool.dispatch()

	return &pool
}

//func (pool *ActorPool) Start() error {
//
//	atomic.SwapUint32(&pool.state, STARTING)
//	for _, worker := range pool.Workers {
//		if initErr := pool.init(worker, 5); initErr != nil {
//			return initErr
//		}
//	}
//
//	atomic.SwapUint32(&pool.state, STARTED)
//	return nil
//}

//func (pool *ActorPool) Shutdown() {
//
//}
//
//
//func (pool *ActorPool) Execute(ops []Operation) <-chan Operation {
//	for atomic.LoadInt32(&pool.liveliness) < 0 {}
//	output := make(chan Operation, len(ops))
//	wg := sync.WaitGroup{}
//
//	for _, op := range ops {
//		pool.requestChan <- op
//		wg.Add(1)
//		go func(op Operation) {
//			defer func() {
//				if recover() != nil {
//					op.Done()
//					output <- op
//					wg.Done()
//				}
//			}()
//
//			op.Wait()
//			output <- op
//			wg.Done()
//		}(op)
//	}
//
//	go func() {
//		wg.Wait()
//		close(output)
//	}()
//
//	return output
//}
//
//
//func (pool *ActorPool) restore(worker Worker) {
//	pool.init(worker, 5)
//}
//
//func (pool *ActorPool) init(worker Worker, tries uint) error {
//	initErr := worker.Init()
//	for initErr != nil && tries > uint(0) {
//		tries -= 1
//		initErr = worker.Init()
//	}
//
//	if initErr != nil {
//		pool.Shutdown()
//		return initErr
//	}
//
//	go func() {
//		// In case there is a panic, let's restart the worker
//		// but otherwise, just clean up the worker, however it
//		// knows how
//		var currentOp Operation
//		defer func() {
//			if currentOp != nil {
//				// not retrying because the process clearly is shutting
//				// down
//				currentOp.Done()
//			}
//
//			worker.Cleanup()
//			if r := recover(); r != nil {
//				pool.restore(worker)
//			}
//		}()
//
//		for op := range pool.requestChan {
//			currentOp = op
//			retry, err := worker.Process(op)
//			if err != nil {
//				if retry {
//					op.IncrementTry()
//
//					// Tricky, tricky! If you push this back on
//					// queue, it will block, preventing this processor
//					// from being able to retrieve message. Cannot block on
//					// putting this message back on queue
//					go func() {
//						defer func() {
//							if r := recover(); r != nil {
//								// TODO: do something here: we have failed to redrive msg
//							}
//						}()
//
//						pool.requestChan <- op
//					}()
//				} else {
//					worker.HandleError(err, op)
//					op.Done()
//				}
//			} else {
//				op.Done()
//			}
//		}
//	}()
//
//	return nil
//}
//

