package cmd

import (
	"bytes"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// slowWriter is an io.Writer that blocks for a configurable duration
// on each Write call, simulating slow I/O to force buffer saturation.
type slowWriter struct {
	mu      sync.Mutex
	buf     bytes.Buffer
	delay   time.Duration
	written int
}

func (sw *slowWriter) Write(p []byte) (int, error) {
	if sw.delay > 0 {
		time.Sleep(sw.delay)
	}
	sw.mu.Lock()
	defer sw.mu.Unlock()
	sw.written++
	return sw.buf.Write(p)
}

func (sw *slowWriter) String() string {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	return sw.buf.String()
}

func TestAsyncWriter_NormalWrite(t *testing.T) {
	var buf bytes.Buffer
	aw := newAsyncWriter(&buf, 100)

	messages := []string{
		"hello world\n",
		"second message\n",
		"third message\n",
	}

	for _, msg := range messages {
		n, err := aw.Write([]byte(msg))
		require.NoError(t, err)
		assert.Equal(t, len(msg), n)
	}

	err := aw.Close()
	require.NoError(t, err)

	output := buf.String()
	for _, msg := range messages {
		assert.Contains(t, output, msg, "output should contain the written message")
	}

	assert.Equal(t, uint64(0), aw.Dropped(), "no messages should have been dropped")
}

func TestAsyncWriter_BufferFull_Drops(t *testing.T) {
	// Use a slow writer and tiny buffer to force drops
	sw := &slowWriter{delay: 10 * time.Millisecond}
	bufferSize := 5
	aw := newAsyncWriter(sw, bufferSize)

	// Write many more messages than the buffer can hold.
	// The slow writer ensures the consumer goroutine cannot drain
	// the buffer fast enough, so the non-blocking send will hit
	// the default case and increment the dropped counter.
	totalMessages := 200
	for i := 0; i < totalMessages; i++ {
		aw.Write([]byte("msg\n")) //nolint:errcheck
	}

	err := aw.Close()
	require.NoError(t, err)

	dropped := aw.Dropped()
	assert.Greater(t, dropped, uint64(0), "some messages should have been dropped")

	// Verify that the drop report notice was written
	output := sw.String()
	assert.Contains(t, output, "[asyncWriter] dropped", "drop notice should appear in output")
}

func TestAsyncWriter_Close(t *testing.T) {
	var buf bytes.Buffer
	aw := newAsyncWriter(&buf, 100)

	// Write one message before close
	n, err := aw.Write([]byte("before close\n"))
	require.NoError(t, err)
	assert.Equal(t, len("before close\n"), n)

	err = aw.Close()
	require.NoError(t, err)

	// Subsequent writes should return os.ErrClosed
	n, err = aw.Write([]byte("after close\n"))
	assert.Equal(t, 0, n)
	assert.ErrorIs(t, err, os.ErrClosed)

	// Double close should be safe
	err = aw.Close()
	assert.NoError(t, err)
}

func TestAsyncWriter_DroppedCounter(t *testing.T) {
	// Verify Dropped() returns zero for a fresh writer
	var buf bytes.Buffer
	aw := newAsyncWriter(&buf, 100)
	assert.Equal(t, uint64(0), aw.Dropped())
	err := aw.Close()
	require.NoError(t, err)
}

func TestAsyncWriter_FinalDropReport(t *testing.T) {
	// Use a slow writer and tiny buffer to force drops, but fewer than
	// dropReportInterval so the final flush branch is exercised.
	sw := &slowWriter{delay: 5 * time.Millisecond}
	bufferSize := 2
	aw := newAsyncWriter(sw, bufferSize)

	// Send enough to drop some, but aim for fewer than 100 drops
	// so the final report fires (drops % 100 != 0).
	for i := 0; i < 50; i++ {
		aw.Write([]byte("msg\n")) //nolint:errcheck
	}

	err := aw.Close()
	require.NoError(t, err)

	dropped := aw.Dropped()
	if dropped > 0 && dropped%dropReportInterval != 0 {
		output := sw.String()
		assert.True(t, strings.Contains(output, "total (final)"),
			"final drop report should be written when drops are not a multiple of the interval")
	}
}

func TestAsyncWriter_CopiesInput(t *testing.T) {
	// Verify that the writer copies the input buffer so callers
	// can safely reuse the slice.
	var buf bytes.Buffer
	aw := newAsyncWriter(&buf, 100)

	data := []byte("original")
	_, err := aw.Write(data)
	require.NoError(t, err)

	// Mutate the original slice after writing
	copy(data, "modified")

	err = aw.Close()
	require.NoError(t, err)

	assert.Contains(t, buf.String(), "original",
		"writer should have used a copy, not the original slice")
	assert.NotContains(t, buf.String(), "modified",
		"mutation of input slice should not affect written data")
}

// TestAsyncWriter_ConcurrentWriteClose stresses concurrent Write vs Close. Under the
// race detector it must not report a data race on the closed flag, and it must never
// panic with "send on closed channel" (a Write that passes the closed check before
// Close runs must not then send on a closed channel). Writes after Close return
// os.ErrClosed.
func TestAsyncWriter_ConcurrentWriteClose(t *testing.T) {
	for i := 0; i < 50; i++ {
		var buf syncBuffer
		aw := newAsyncWriter(&buf, 4)

		var wg sync.WaitGroup
		for w := 0; w < 8; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 50; j++ {
					// A returned error (os.ErrClosed) is fine; a panic is not.
					_, _ = aw.Write([]byte("log line\n"))
				}
			}()
		}

		// Close concurrently with the writers.
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = aw.Close()
		}()

		wg.Wait()

		// Writes after close must report closed, not panic.
		_, err := aw.Write([]byte("after close\n"))
		assert.ErrorIs(t, err, os.ErrClosed)
	}
}

// syncBuffer is a minimal concurrency-safe io.Writer for the stress test (the
// asyncWriter goroutine writes to it while the test may read).
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}
