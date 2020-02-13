package bulk

import (
	"errors"
	"time"
)

// A function of a FlushFunc type once called will receive
// a buffer containing all the data from writes made after
// the previous FlushFunc call. The data buffer will be cleaned up
// automatically after this function is executed (i.e. you do
// not need to clean it up yourself).
// Any error returned from this function will be passed to a ErrorHandlerFunc function
type FlushFunc func(data []byte) error

// ErrorHandlerFunc is a function that gets errors occured in FlushFunc
// It will also get a buffer copy so that you could somehow analyze it.
// After this function is called, the buffer is destroyed automatically
// by the calling code (i.e. you do not need to clean it up yourself).
type ErrorHandlerFunc func(data []byte, err error)

// NoErrorHandler is an empty function that is used when no ErrorHandler is required
// One may always process all their errors directly in the FlushFunc.
var NoErrorHandler = func(data []byte, err error) {}

// Writer is an implenetation of an io.WriteCloser interface.
// It lets creating a buffered writer that can flush (and thus physically write)
// the buffer by a time ticker or by manual calls of Writer.Flush().
type Writer struct {
	ticker       *time.Ticker
	tickerCh     <-chan time.Time
	buf          []byte
	data         chan []byte
	quit         chan bool
	flusher      chan bool
	closed       bool
	flushFunc    FlushFunc
	errorHandler ErrorHandlerFunc
}

// NewBulkWriter creates a new bulk.Writer instance
// flushInterval - how often to call the flushFunc, if set to a nonpositive value will effectively turn
//                 off automatic flushing
// flushFunc - defines what to do on flush
func NewBulkWriter(flushInterval time.Duration, flushFunc FlushFunc) *Writer {
	return NewBulkWriterWithErrorHandler(flushInterval, flushFunc, NoErrorHandler)
}

// NewBulkWriterWithErrorHandler creates a new bulk.Writer instance
// flushInterval - how often to call the flushFunc, if set to a nonpositive value will effectively turn
//                 off automatic flushing
// flushFunc - defines what to do on flush
// errorHandler - whenever your flushFunc returns an error, it can be processed in this function
func NewBulkWriterWithErrorHandler(flushInterval time.Duration, flushFunc FlushFunc, errorHandler ErrorHandlerFunc) *Writer {
	bw := &Writer{
		buf:          make([]byte, 0, 0),
		data:         make(chan []byte),
		quit:         make(chan bool),
		flushFunc:    flushFunc,
		errorHandler: errorHandler,
		flusher:      make(chan bool),
	}
	if flushInterval > 0 {
		bw.ticker = time.NewTicker(flushInterval)
		bw.tickerCh = bw.ticker.C
	} else {
		bw.tickerCh = make(chan time.Time)
	}
	go bw.processor()
	return bw
}

func (b *Writer) flush() {
	if len(b.buf) == 0 {
		return
	}
	if err := b.flushFunc(b.buf); err != nil {
		b.errorHandler(b.buf, err)
	}
	b.buf = []byte{}
}

func (b *Writer) processor() {
loop:
	for {
		select {
		case d := <-b.data:
			b.buf = append(b.buf, d...)
		case <-b.flusher:
			b.flush()
		case <-b.tickerCh:
			b.flush()
		case <-b.quit:
			b.flush()
			break loop
		}
	}
}

// Write is an implementation of an io.Writer interface. The data are appended to a temporary
// buffer that will be cleaned up on flush.
// It will return an error if called after Close() was called.
func (b *Writer) Write(data []byte) (n int, err error) {
	if b.closed {
		return 0, errors.New("writing on a closed bulk.Writer")
	}

	b.data <- data

	return len(data), nil
}

// Flush forces buffer flush. It is mainly suited for buffer flushing
// when automatic flushing is turned off, but you may call it even
// if automatic flushing is turned on.
// It will return an error if called after Close() was called.
func (b *Writer) Flush() error {
	if b.closed {
		return errors.New("flushing a closed bulk.Writer")
	}
	b.flusher <- true
	return nil
}

// Close is an implementation of an io.Closer interface.
// It closes the writer, stops any activity and any subsiquent operations
// will result in a error.
// It will return an error if called after Close() was called.
func (b *Writer) Close() error {
	if b.closed {
		return errors.New("closing a closed bulk.Writer")
	}

	b.closed = true
	close(b.quit)
	if b.ticker != nil {
		b.ticker.Stop()
	}
	return nil
}
