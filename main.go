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
	"os"

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
	asyncWriter := newAsyncWriter(f, 1000) // Buffer up to 1000 log messages
	defer asyncWriter.Close()

	log.SetOutput(asyncWriter)

	cmd.Execute()
}

// asyncWriter wraps an io.Writer and writes asynchronously via a channel
type asyncWriter struct {
	out    chan []byte
	done   chan struct{}
	closed bool
}

func newAsyncWriter(w *os.File, bufferSize int) *asyncWriter {
	aw := &asyncWriter{
		out:  make(chan []byte, bufferSize),
		done: make(chan struct{}),
	}

	// Start background goroutine to write logs
	go func() {
		for msg := range aw.out {
			w.Write(msg) //nolint:errcheck
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
		return len(p), nil
	}
}

func (aw *asyncWriter) Close() error {
	if !aw.closed {
		aw.closed = true
		close(aw.out)
		<-aw.done // Wait for goroutine to finish
	}
	return nil
}
