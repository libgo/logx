package logx

import (
	"context"
	"io"
	"sync"
	"time"

	diodes "code.cloudfoundry.org/go-diodes"
)

var bufPool = &sync.Pool{
	New: func() interface{} {
		return make([]byte, 0, 500)
	},
}

type Alerter func(missed int)

type diodeFetcher interface {
	diodes.Diode
	Next() diodes.GenericDataType
}

// Writer is a io.Writer wrapper that uses a diode to make Write lock-free,
// non-blocking and thread safe.
type AsyncWriter struct {
	lv   Level
	w    Writer
	d    diodeFetcher
	c    context.CancelFunc
	done chan struct{}
}

// NewAsyncWriter creates a writer wrapping w with a many-to-one diode in order to
// never block log producers and drop events if the writer can't keep up with
// the flow of data.
//
// Use a diode.Writer when
//
//     wr := diode.NewWriter(w, 1000, 10 * time.Millisecond, func(missed int) {
//         log.Printf("Dropped %d messages", missed)
//     })
//     log := zerolog.New(wr)
//
//
// See code.cloudfoundry.org/go-diodes for more info on diode.
func NewAsyncWriter(l Level, w Writer, size int, poolInterval time.Duration, f Alerter) AsyncWriter {
	ctx, cancel := context.WithCancel(context.Background())
	dw := AsyncWriter{
		lv:   l,
		w:    w,
		c:    cancel,
		done: make(chan struct{}),
	}
	if f == nil {
		f = func(int) {}
	}
	d := diodes.NewManyToOne(size, diodes.AlertFunc(f))
	if poolInterval > 0 {
		dw.d = diodes.NewPoller(d,
			diodes.WithPollingInterval(poolInterval),
			diodes.WithPollingContext(ctx))
	} else {
		dw.d = diodes.NewWaiter(d,
			diodes.WithWaiterContext(ctx))
	}
	go dw.poll()
	return dw
}

func (dw AsyncWriter) Write(p []byte) (n int, err error) {
	// p is pooled in zerolog so we can't hold it passed this call, hence the
	// copy.
	p = append(bufPool.Get().([]byte), p...)
	dw.d.Set(diodes.GenericDataType(&p))
	return len(p), nil
}

func (dw AsyncWriter) WriteLevel(level Level, p []byte) (n int, err error) {
	if level < dw.lv {
		return len(p), nil
	}

	return dw.Write(p)
}

// Close releases the diode poller and call Close on the wrapped writer if
// io.Closer is implemented.
func (dw AsyncWriter) Close() error {
	dw.c()
	<-dw.done
	if w, ok := dw.w.(io.Closer); ok {
		return w.Close()
	}
	return nil
}

func (dw AsyncWriter) poll() {
	defer close(dw.done)
	for {
		d := dw.d.Next()
		if d == nil {
			return
		}
		p := *(*[]byte)(d)
		dw.w.Write(p)

		// Proper usage of a sync.Pool requires each entry to have approximately
		// the same memory cost. To obtain this property when the stored type
		// contains a variably-sized buffer, we add a hard limit on the maximum buffer
		// to place back in the pool.
		//
		// See https://golang.org/issue/23199
		const maxSize = 1 << 16 // 64KiB
		if cap(p) <= maxSize {
			bufPool.Put(p[:0])
		}
	}
}
