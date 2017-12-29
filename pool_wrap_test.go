package harbinger

import (
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

	testWithTimeout(t, func(t *testing.T){
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
	}, 1 * time.Second)
}