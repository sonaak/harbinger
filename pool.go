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
// The interface assumes that Wait() would block until the operation
// is Done(). Otherwise, the operation may be a bit unpredictable.
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

// Worker - an interface representing a client or processor that
// handles requests (operations).
//
// For example, if you want to define a pool of map/reduce processes
// then a worker may be a logical unit (consisting of several logical
// units) that processes a request. If you want to define a pool of
// database clients, then a worker may be some struct that accepts a
// database query for an op, and returns the seeker for the row.
//
// Such a client/processor should know how to handle errors and clean
// up. The main function here would be its Process function which
// processes a certain operation and returns if it failed and if the
// operation should be retried.
type Worker interface {

	// Init -
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
	STOPPING uint32 = 4
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
	go pool.Shutdown()
	return errors.Wrap(initErr, "unable to initialise worker")
}

func (pool *ActorPool) start() error {
	// if the pool is already running, don't do anything
	if pool.state == RUNNING || pool.state == STARTING {
		return nil
	}

	pool.state = STARTING

	// create brand new operations channel and close-once sema
	pool.operationChan = make(chan Operation)
	pool.closeOnce = &sync.Once{}

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

	pool.state = STOPPING
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
			v.Error = pool.start()
			v.Done()
			if v.Error == nil {
				pool.state = RUNNING
			}

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
			pool.shutdown()
			pool.state = STOPPED
		}
	}
}

func NewPool(workers []Worker) *ActorPool {
	pool := ActorPool{
		Workers:       workers,
		reqChan:       make(chan poolreq),
		execWg:        &sync.WaitGroup{},
		state:         STOPPED,
	}

	go pool.listenToRequests()
	return &pool
}

func (pool *ActorPool) Start() error {
	req := &StartupRequest{
		AsyncRequest{
			done: make(chan interface{}),
		},
	}
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
