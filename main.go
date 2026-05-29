/*
Copyright © 2023 Chris Collins 'collins.christopher@gmail.com'

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
package main

import (
	"fmt"
	"io"
	"os"
	"sync/atomic"

	"github.com/charmbracelet/log"
	"github.com/clcollins/srepd/cmd"
)

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}

	// Open log file, truncating if it exists to prevent unbounded growth
	// TODO: Implement proper log rotation
	f, err := os.OpenFile(home+"/.config/srepd/debug.log", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600) //nolint:gomnd
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close() //nolint:errcheck

	// Use async writer to prevent log I/O from blocking the UI
	asyncWriter := newAsyncWriter(f, 5000) // Buffer up to 5000 log messages
	defer asyncWriter.Close() //nolint:errcheck

	log.SetOutput(asyncWriter)

	cmd.Execute()
}

// asyncWriter wraps an io.Writer and writes asynchronously via a channel.
// When the internal buffer is full, messages are dropped and counted.
// The background goroutine periodically emits a synthetic log entry
// reporting how many messages were dropped.
type asyncWriter struct {
	out     chan []byte
	done    chan struct{}
	closed  bool
	dropped uint64 // accessed atomically
}

// dropReportInterval controls how often a synthetic "dropped N messages"
// entry is written to the underlying writer. A report is emitted every
// time the cumulative drop count crosses a multiple of this value.
const dropReportInterval uint64 = 100

func newAsyncWriter(w io.Writer, bufferSize int) *asyncWriter {
	aw := &asyncWriter{
		out:  make(chan []byte, bufferSize),
		done: make(chan struct{}),
	}

	// Start background goroutine to write logs
	go func() {
		var lastReported uint64
		for msg := range aw.out {
			w.Write(msg) //nolint:errcheck

			// Check for dropped messages and emit a report periodically
			current := atomic.LoadUint64(&aw.dropped)
			if current > 0 && current/dropReportInterval > lastReported/dropReportInterval {
				notice := fmt.Sprintf("[asyncWriter] dropped %d log messages due to full buffer\n", current)
				w.Write([]byte(notice)) //nolint:errcheck
				lastReported = current
			}
		}

		// Flush any remaining drop count on shutdown that wasn't
		// covered by the last periodic report
		finalDropped := atomic.LoadUint64(&aw.dropped)
		if finalDropped > 0 && finalDropped%dropReportInterval != 0 {
			notice := fmt.Sprintf("[asyncWriter] dropped %d log messages total (final)\n", finalDropped)
			w.Write([]byte(notice)) //nolint:errcheck
		}

		close(aw.done)
	}()

	return aw
}

func (aw *asyncWriter) Write(p []byte) (n int, err error) {
	if aw.closed {
		return 0, os.ErrClosed
	}

	// Make a copy since the caller might reuse the buffer
	msg := make([]byte, len(p))
	copy(msg, p)

	// Non-blocking send - if buffer is full, drop the message
	// This prevents blocking the UI if logging falls behind
	select {
	case aw.out <- msg:
		return len(p), nil
	default:
		// Buffer full - drop message to avoid blocking
		atomic.AddUint64(&aw.dropped, 1)
		return len(p), nil
	}
}

// Dropped returns the number of messages dropped due to a full buffer.
func (aw *asyncWriter) Dropped() uint64 {
	return atomic.LoadUint64(&aw.dropped)
}

func (aw *asyncWriter) Close() error {
	if !aw.closed {
		aw.closed = true
		close(aw.out)
		<-aw.done // Wait for goroutine to finish
	}
	return nil
}
