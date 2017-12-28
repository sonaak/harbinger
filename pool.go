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
	wrap     reqtype = 4
	dosingle reqtype = 5

	stopped  uint32 = 0
	starting uint32 = 1
	running  uint32 = 2
	stopping uint32 = 4
)

type poolreq interface {
	Type() reqtype
}

type asyncreq struct {
	Error error

	done chan interface{}
}

func (req *asyncreq) Wait() {
	for range req.done {
	}
}

func (req *asyncreq) Done() {
	defer func() {
		if r := recover(); r != nil {
		}
	}()

	close(req.done)
}

type startupReq struct {
	asyncreq
}

func (req *startupReq) Type() reqtype {
	return startup
}

type shutdownReq bool

func (req shutdownReq) Type() reqtype {
	return shutdown
}

type executeReq struct {
	asyncreq
	Operations []Operation
	Output     chan<- Operation
}

func (req *executeReq) Type() reqtype {
	return execute
}

type doSingleReq struct {
	asyncreq
	Input Operation
}


func (doSingleReq) Type() reqtype {
	return dosingle
}


type wrapStreamReq struct {
	asyncreq
	Input <-chan Operation
	Output chan Operation
}

func (req *wrapStreamReq) Type() reqtype {
	return wrap
}


type redriveReq struct {
	asyncreq
	PreviousAssignee Worker
	Operation        Operation
}

func (req *redriveReq) Type() reqtype {
	return redrive
}

// WorkerPool - represents a pool of (possibly heterogeneous) Workers who will read
// messages off of a queue and process them. The idea here is that the messages
// without curation and dispatch may go through several passes before being handled
// by the correct worker.
//
// What the worker pool does, then, is to provide basic controls around spinning up
// the worker, assigning tasks to the worker, keeping the workers up, and shutting
// down the pool when everything is done.
//
// It exposes just three methods: Start, Execute, Shutdown. With these three methods
// we should be able to push tasks to initialise the workers, push tasks to them, and
// reclaim resources when done.
type WorkerPool struct {
	Workers []Worker

	operationChan chan Operation
	reqChan       chan poolreq
	state         uint32
	execWg        *sync.WaitGroup
	closeOnce     *sync.Once
}

func (pool *WorkerPool) restore(worker Worker) {
	// TODO: do other restore-y things
	pool.runWorkerAsListener(worker)
}

func (pool *WorkerPool) retryOperation(op Operation, previousAssignee Worker) {
	req := redriveReq{
		Operation:        op,
		PreviousAssignee: previousAssignee,
	}

	pool.reqChan <- &req
}

func (pool *WorkerPool) assign(worker Worker, op Operation) {
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

func (pool *WorkerPool) runWorkerAsListener(worker Worker) {
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

func (pool *WorkerPool) initWorker(worker Worker) error {
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

func (pool *WorkerPool) start() error {
	// if the pool is already running, don't do anything
	if pool.state == running || pool.state == starting {
		return nil
	}

	pool.state = starting

	// create brand new operations channel and close-once sema
	pool.operationChan = make(chan Operation)
	pool.closeOnce = &sync.Once{}
	pool.execWg = &sync.WaitGroup{}

	for _, worker := range pool.Workers {
		if initErr := pool.initWorker(worker); initErr != nil {
			return initErr
		}
		go pool.runWorkerAsListener(worker)
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

func (pool *WorkerPool) enqueue(ops []Operation, output chan<- Operation) error {
	if pool.state != running {
		close(output)
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

func (pool *WorkerPool) do(op Operation) error {
	if pool.state != running {
		return errors.New("error: enqueuing message on non-running actor pool")
	}

	pool.operationChan <- op
	return nil
}

func (pool *WorkerPool) shutdown() {
	// if the pool is already shutdown, just return
	if pool.state == stopped {
		return
	}

	pool.state = stopping
	// otherwise, close the operation channel in a once
	pool.closeOnce.Do(func() {
		close(pool.operationChan)
	})
}

func (pool *WorkerPool) redrive(op Operation, previousWorker Worker) {
	if pool.state != running {
		pool.assign(previousWorker, op)
	}

	pool.operationChan <- op
}

func (pool *WorkerPool) listenToRequests() {
	for req := range pool.reqChan {
		switch v := req.(type) {

		case *startupReq:
			v.Error = pool.start()
			v.Done()
			if v.Error == nil {
				pool.state = running
			}

		case *executeReq:
			pool.execWg.Add(1)
			go func() {
				v.Error = pool.enqueue(v.Operations, v.Output)
				v.Done()
				pool.execWg.Done()
			}()

		case *doSingleReq:
			pool.execWg.Add(1)
			go func() {
				v.Error = pool.do(v.Input)
				v.Done()
				pool.execWg.Done()
			}()

		case *redriveReq:
			pool.redrive(v.Operation, v.PreviousAssignee)

		case *wrapStreamReq:
			pool.execWg.Add(1)
			go func() {
				outStream, err := pool.wrapStream(v.Input, pool.execWg)
				v.Error = err
				v.Output = outStream
				v.Done()
			}()

		case shutdownReq:
			// wait till all the previous executes have completed
			pool.execWg.Wait()

			// then stop everything
			pool.shutdown()
			pool.state = stopped
		}
	}
}

// NewPool - creates a new pool of workers. By passing it a list of
// workers, each will be initialised, and registered to receive messages
// on a queue, and restarted when some error occurs. Each will be shutdown
// appropriately when the shutdown sequence is called.
func NewPool(workers []Worker) *WorkerPool {
	pool := WorkerPool{
		Workers: workers,
		reqChan: make(chan poolreq),
		state:   stopped,
		execWg:  &sync.WaitGroup{},
	}

	go pool.listenToRequests()
	return &pool
}

// Start - start the pool by setting up the workers to listen to the
// requests. A pool that isn't started cannot process any requests.
func (pool *WorkerPool) Start() error {
	req := &startupReq{
		asyncreq{
			done: make(chan interface{}),
		},
	}
	pool.reqChan <- req

	// wait till the request is done
	req.Wait()
	return req.Error
}

// Shutdown - shuts down the actor pool so that (some of) its resources
// can be reused, and the workers will be notified to shutdown.
//
// This will prevent all other requests from being executed. This does
// not affect any requests that have begun processing.
func (pool *WorkerPool) Shutdown() {
	pool.reqChan <- shutdownReq(true)
}

// Execute - executes a collection of operations (requests). The return
// is a channel that is closed when all the operations have either been
// successfully processed or failed.
//
// If an error is returned, the channel is closed.
func (pool *WorkerPool) Execute(ops []Operation) (<-chan Operation, error) {
	output := make(chan Operation, len(ops))
	executeReq := executeReq{
		Operations: ops,
		Output:     output,
		asyncreq: asyncreq{
			done: make(chan interface{}),
		},
	}
	pool.reqChan <- &executeReq
	executeReq.Wait()
	return output, executeReq.Error
}


// Do - executes a single operation asynchronously. This returns an error
// if the worker pool is not running (e.g. has not started or is shutdown)
// because then the channels are all closed.
func (pool *WorkerPool) Do(op Operation) error {
	doSingleReq := doSingleReq {
		Input: op,
	}

	pool.reqChan <- &doSingleReq
	doSingleReq.Wait()
	return doSingleReq.Error
}

func (pool *WorkerPool) pipe(inStream <-chan Operation, outStream chan Operation, wg *sync.WaitGroup) {
	taskWg := sync.WaitGroup {}
	for op := range inStream {
		// execute the op
		doErr := pool.Do(op)
		if doErr != nil {
			// TODO: do some error handling here
			op.Done()
		}

		taskWg.Add(1)

		// wait till the op is done, and then move it to the outstream
		go func(op Operation, taskWg *sync.WaitGroup) {
			op.Wait()
			outStream <- op
			taskWg.Done()
		}(op, &taskWg)
	}

	// even if the inStream is closed, wait till all the
	// tasks have completed before returning
	taskWg.Wait()

	// at this point, mark the whole execute as done
	wg.Done()

	// close the outstream
	close(outStream)
}

func (pool *WorkerPool) wrapStream(inStream <-chan Operation, wg *sync.WaitGroup) (chan Operation, error) {
	outStream := make(chan Operation)
	if pool.state != running {
		close(outStream)
		return outStream, errors.New("pool not running; stream will never be processed")
	}

	go pool.pipe(inStream, outStream, wg)
	return outStream, nil
}

// Wrap - wraps an input stream into another output stream. The idea here
// is if the client has a streaming input, then rather than forcing clients
// to batch and executing using Execute, they should be able to fire directly into
// an input channel and we will handle processing the operations and then as
// operations are done, putting them onto the output channel.
//
// When the input stream is closed, the output stream may not close. The output
// stream is only closed when all the inbound requests have been handled, either
// successfully or unsuccessfully (but marked as done).
func (pool *WorkerPool) Wrap(inStream <- chan Operation) (<-chan Operation, error) {
	req := wrapStreamReq {
		Input: inStream,
		asyncreq: asyncreq{
			done: make(chan interface{}),
		},
	}

	pool.reqChan <- &req
	req.Wait()
	return req.Output, req.Error
}
