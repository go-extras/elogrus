package bulk

import (
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

const TestData = "The Answer to the Ultimate Question of Life, the Universe, and Everything."

func TestWriter_Write(t *testing.T) {
	var called int32
	w := NewBulkWriterWithErrorHandler(time.Millisecond,
		func(data []byte) error {
			atomic.AddInt32(&called, 1)
			if string(data) != TestData {
				t.Errorf("Unexpected data: %q", string(data))
				t.FailNow()
			}
			return nil
		},
		func(data []byte, err error) {
			t.Error("Unexpected error handler call")
			t.FailNow()
		},
	)
	n, err := w.Write([]byte(TestData))
	if err != nil {
		t.Errorf("Error closing the writer: %s", err.Error())
		t.FailNow()
	}
	if n != len(TestData) {
		t.Errorf("unexpected length: %d", n)
		t.FailNow()
	}

	time.Sleep(10 * time.Millisecond)
	err = w.Close()
	if err != nil {
		t.Errorf("Error closing the writer: %s", err.Error())
		t.FailNow()
	}

	if atomic.LoadInt32(&called) == 0 {
		t.Error("FlushFunc was never called")
		t.FailNow()
	}
}

func TestWriter_Flush(t *testing.T) {
	var called int32
	w := NewBulkWriterWithErrorHandler(0, // this lets us avoid the automatic flush call
		func(data []byte) error {
			atomic.AddInt32(&called, 1)
			if string(data) != TestData {
				t.Errorf("Unexpected data: %q", string(data))
				t.FailNow()
			}
			return nil
		},
		func(data []byte, err error) {
			t.Error("Unexpected error handler call")
			t.FailNow()
		},
	)
	w.Write([]byte(TestData))
	time.Sleep(10 * time.Millisecond)
	if atomic.LoadInt32(&called) > 0 {
		t.Error("Flush was unexpectedly called")
		t.FailNow()
	}

	err := w.Flush()
	if err != nil {
		t.Errorf("Error flushing the writer: %s", err.Error())
		t.FailNow()
	}
	runtime.Gosched() // explicitely call other go routines

	if atomic.LoadInt32(&called) == 0 {
		t.Error("FlushFunc was not called")
		t.FailNow()
	}

	err = w.Close()
	if err != nil {
		t.Errorf("Error closing the writer: %s", err.Error())
		t.FailNow()
	}
}
