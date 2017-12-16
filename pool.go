package harbinger

import (
	"github.com/pkg/errors"
	"sync"
)

// Operation - an interface representing a unit of task to be
// done by a worker. For example, if you want to define a pool of
// http requests or database readers, then your operation can be
// a request object or an object that represents a database read.
// To capture the output, either publicly or privately set an output.
//
// The interface assumes that Wait() would
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
	shutdown reqtype = 0
	startup  reqtype = 1
	execute  reqtype = 2
	redrive  reqtype = 3

	STOPPED  uint32 = 0
	STARTING uint32 = 1
	RUNNING  uint32 = 2
)

type poolreq interface {
	Type() reqtype
}

type AsyncRequest struct {
	Error error

	done chan interface{}
}

func (req *AsyncRequest) Wait() {
	for range req.done {
	}
}

func (req *AsyncRequest) Done() {
	close(req.done)
}

type StartupRequest struct {
	AsyncRequest
}

func (req *StartupRequest) Type() reqtype {
	return startup
}

type ShutdownRequest bool

func (req ShutdownRequest) Type() reqtype {
	return shutdown
}

type ExecuteRequest struct {
	AsyncRequest
	Operations []Operation
	Output     chan<- Operation
}

func (req *ExecuteRequest) Type() reqtype {
	return execute
}

type RedriveRequest struct {
	AsyncRequest
	PreviousAssignee Worker
	Operation        Operation
}

func (req *RedriveRequest) Type() reqtype {
	return redrive
}

type ActorPool struct {
	Workers []Worker

	operationChan chan Operation
	reqChan       chan poolreq
	state         uint32
	execWg        *sync.WaitGroup
	closeOnce     *sync.Once
}

func (pool *ActorPool) restore(worker Worker) {
	// TODO: do other restore-y things
	pool.register(worker)
}

func (pool *ActorPool) retryOperation(op Operation, previousAssignee Worker) {
	req := RedriveRequest{
		Operation:        op,
		PreviousAssignee: previousAssignee,
	}

	pool.reqChan <- &req
}

func (pool *ActorPool) assign(worker Worker, op Operation) {
	retry, err := worker.Process(op)
	if err == nil {
		op.Done()
		return
	}

	if retry {
		op.IncrementTry()
		pool.retryOperation(op, worker)
	} else {
		worker.HandleError(err, op)
		op.Done()
	}
}

func (pool *ActorPool) register(worker Worker) {
	// In case there is a panic, let's restart the worker
	// but otherwise, just clean up the worker, however it
	// knows how
	var currentOp Operation
	defer func() {
		if currentOp != nil {
			currentOp.Done()
		}

		worker.Cleanup()
		if r := recover(); r != nil {
			pool.restore(worker)
		}
	}()

	// process each operation received operation by assigning
	// to this worker
	for currentOp = range pool.operationChan {
		pool.assign(worker, currentOp)
	}
}

func (pool *ActorPool) initWorker(worker Worker) error {
	initErr := worker.Init()

	if initErr == nil {
		return nil
	}

	// if there is any initialisation errors,
	// we should shutdown the pool immediately so it can't
	// be used
	pool.Shutdown()
	return errors.Wrap(initErr, "unable to initialise worker")
}

func (pool *ActorPool) start() error {
	// if the pool is already running, don't do anything
	if pool.state == RUNNING {
		return nil
	}

	for _, worker := range pool.Workers {
		if initErr := pool.initWorker(worker); initErr != nil {
			return initErr
		}
		go pool.register(worker)
	}

	return nil
}

func waitTillCompleted(wg *sync.WaitGroup, op Operation, output chan<- Operation) {
	defer func() {
		if recover() != nil {
			op.Done()
			output <- op
			wg.Done()
		}
	}()

	op.Wait()
	output <- op
	wg.Done()
}

func (pool *ActorPool) enqueue(ops []Operation, output chan<- Operation) error {
	if pool.state != RUNNING {
		return errors.New("error: enqueuing messages on non-running actor pool")
	}

	wg := sync.WaitGroup{}
	for _, op := range ops {
		pool.operationChan <- op
		wg.Add(1)
		go waitTillCompleted(&wg, op, output)
	}

	go func() {
		wg.Wait()
		close(output)
	}()

	return nil
}

func (pool *ActorPool) shutdown() {
	// if the pool is already shutdown, just return
	if pool.state == STOPPED {
		return
	}

	// otherwise, close the operation channel in a once
	pool.closeOnce.Do(func() {
		close(pool.operationChan)
	})
}

func (pool *ActorPool) redrive(op Operation, previousWorker Worker) {
	if pool.state != RUNNING {
		pool.assign(previousWorker, op)
	}

	pool.operationChan <- op
}

func (pool *ActorPool) listenToRequests() {
	for req := range pool.reqChan {
		switch v := req.(type) {

		case *StartupRequest:
			pool.state = STARTING
			v.Error = pool.start()
			v.Done()
			pool.state = RUNNING

		case *ExecuteRequest:
			pool.execWg.Add(1)
			go func() {
				v.Error = pool.enqueue(v.Operations, v.Output)
				v.Done()
				pool.execWg.Done()
			}()

		case *RedriveRequest:
			pool.redrive(v.Operation, v.PreviousAssignee)

		case ShutdownRequest:
			// wait till all the previous executes have completed
			pool.execWg.Wait()

			// then stop everything
			pool.state = STOPPED
			pool.shutdown()

		}
	}
}

func NewPool(workers []Worker) *ActorPool {
	pool := ActorPool{
		Workers:       workers,
		operationChan: make(chan Operation),
		reqChan:       make(chan poolreq),
		state:         STOPPED,
	}

	go pool.listenToRequests()
	return &pool
}

func (pool *ActorPool) Start() error {
	req := &StartupRequest{}
	pool.reqChan <- req

	// wait till the request is done
	req.Wait()
	return req.Error
}

func (pool *ActorPool) Shutdown() {
	pool.reqChan <- ShutdownRequest(true)
}

func (pool *ActorPool) Execute(ops []Operation) (<-chan Operation, error) {
	output := make(chan Operation, len(ops))
	executeReq := ExecuteRequest{
		Operations: ops,
		Output:     output,
	}
	pool.reqChan <- &executeReq
	executeReq.Wait()
	return output, executeReq.Error
}
