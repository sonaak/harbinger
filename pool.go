package harbinger

import (
	"sync"
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

type ActorPool struct {
	Workers []Worker

	requestChan  chan Operation
	shutdownOnce *sync.Once
}

func NewPool(workers []Worker) *ActorPool {
	return &ActorPool{
		Workers:      workers,
		requestChan:  make(chan Operation),
		shutdownOnce: &sync.Once{},
	}
}

func (pool *ActorPool) Shutdown() {
	pool.shutdownOnce.Do(func() {
		close(pool.requestChan)
	})
}

func (pool *ActorPool) restore(worker Worker) {
	pool.init(worker, 5)
}

func (pool *ActorPool) init(worker Worker, tries uint) error {
	initErr := worker.Init()
	for initErr != nil && tries > uint(0) {
		tries -= 1
		initErr = worker.Init()
	}

	if initErr != nil {
		pool.Shutdown()
		return initErr
	}

	go func() {
		// In case there is a panic, let's restart the worker
		// but otherwise, just clean up the worker, however it
		// knows how
		var currentOp Operation
		defer func() {
			if currentOp != nil {
				// not retrying because the process clearly is shutting
				// down
				currentOp.Done()
			}

			worker.Cleanup()
			if r := recover(); r != nil {
				pool.restore(worker)
			}
		}()

		for op := range pool.requestChan {
			currentOp = op
			retry, err := worker.Process(op)
			if err != nil {
				if retry {
					op.IncrementTry()

					// Tricky, tricky! If you push this back on
					// queue, it will block, preventing this processor
					// from being able to retrieve message. Cannot block on
					// putting this message back on queue
					go func() {
						defer func() {
							if r := recover(); r != nil {
								// TODO: do something here: we have failed to redrive msg
							}
						}()

						pool.requestChan <- op
					}()
				} else {
					worker.HandleError(err, op)
					op.Done()
				}
			} else {
				op.Done()
			}
		}
	}()

	return nil
}

func (pool *ActorPool) Start() error {
	for _, worker := range pool.Workers {
		if initErr := pool.init(worker, 5); initErr != nil {
			return initErr
		}
	}

	return nil
}

func (pool *ActorPool) Execute(ops []Operation) <-chan Operation {
	output := make(chan Operation, len(ops))
	wg := sync.WaitGroup{}

	for _, op := range ops {
		pool.requestChan <- op
		wg.Add(1)
		go func(op Operation) {
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
		}(op)
	}

	go func() {
		wg.Wait()
		close(output)
	}()

	return output
}