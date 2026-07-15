package tui

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
	"github.com/clcollins/srepd/pkg/ai"
)

// streamEvent is a single item from a provider stream: either a token of text or a
// terminal signal (done, with an optional error). Exactly one of these is delivered
// per channel receive; the channel is closed after the terminal event.
type streamEvent struct {
	text string
	done bool
	err  error
}

// watcherStreamStartedMsg is emitted when a streaming query has been kicked off. It
// carries the event channel (drained via readStreamCmd) and a cancel func so the
// stream can be aborted (e.g. a new query supersedes it).
type watcherStreamStartedMsg struct {
	ch     <-chan streamEvent
	cancel context.CancelFunc
}

// watcherStreamChunkMsg carries one streamed token to append to the watcher buffer.
type watcherStreamChunkMsg struct {
	text string
	ch   <-chan streamEvent
}

// watcherStreamDoneMsg signals the stream finished (err is nil on success).
type watcherStreamDoneMsg struct {
	err error
}

// streamWatcherCmd starts a streaming provider query. A background goroutine runs
// StreamQuery and forwards each token (and a terminal done event) onto a channel;
// the returned command yields watcherStreamStartedMsg carrying that channel. The
// Update loop then drains it with readStreamCmd. All provider I/O happens in the
// goroutine — the Bubble Tea Update loop is never blocked.
//
// startupTimeout guards only the wait for the FIRST token: once the provider
// starts responding, the watchdog is disarmed and the stream runs to
// completion no matter how long it takes. Whole-stream deadlines would
// truncate long responses mid-token.
//
// Callers must only use this when ai.SupportsStreaming(provider) is true and
// streaming is enabled in config; otherwise use the blocking watcherQueryCmd.
func streamWatcherCmd(provider ai.Provider, systemPrompt, userPrompt, incidentContext string, startupTimeout time.Duration) tea.Cmd {
	return func() tea.Msg {
		// Cancelable only — the startup watchdog below provides the deadline
		// until the first token; after that only the consumer can cancel.
		ctx, cancel := context.WithCancel(context.Background())

		fullPrompt := userPrompt
		if incidentContext != "" {
			fullPrompt = userPrompt + "\n\nContext:\n" + incidentContext
		}

		// Buffered so the producer goroutine is not blocked between Update ticks.
		ch := make(chan streamEvent, 64)

		go func() {
			defer cancel()
			tokens := make(chan string, 64)

			// StreamQuery closes tokens when done; run it and fan tokens into ch.
			errCh := make(chan error, 1)
			go func() {
				errCh <- provider.StreamQuery(ctx, systemPrompt, fullPrompt, tokens)
			}()

			// Startup watchdog: cancel the request if the provider produces
			// nothing before the deadline. Disarmed by the first token.
			var timedOut atomic.Bool
			watchdog := time.AfterFunc(startupTimeout, func() {
				timedOut.Store(true)
				cancel()
			})
			defer watchdog.Stop()

			first := true
			for tok := range tokens {
				if first {
					first = false
					watchdog.Stop()
				}
				select {
				case ch <- streamEvent{text: tok}:
				case <-ctx.Done():
					// Consumer cancelled; drain remaining tokens so StreamQuery's
					// sender does not block, then exit.
					for range tokens {
					}
					ch <- streamEvent{done: true, err: ctx.Err()}
					close(ch)
					return
				}
			}

			err := <-errCh
			if timedOut.Load() {
				// The watchdog cancellation surfaces from StreamQuery as
				// context.Canceled, which the Update loop swallows as the
				// superseded-stream case — replace it with a real timeout error.
				err = fmt.Errorf("no response from provider after %s", startupTimeout)
			}
			if err != nil {
				// Debug, not Warn: the raw error carries the full API response
				// body (a multi-line JSON dump); the classified version is
				// logged at ERROR by errMsgHandler when it reaches the UI.
				log.Debug("watcher.stream", "provider", provider.Name(), "error", err)
			}
			ch <- streamEvent{done: true, err: err}
			close(ch)
		}()

		return watcherStreamStartedMsg{ch: ch, cancel: cancel}
	}
}

// readStreamCmd blocks (off the Update loop, as a tea.Cmd) for the next stream event
// and turns it into a chunk or done message. The chunk handler re-issues this command
// to fetch the following event, draining the channel one event per Update tick.
func readStreamCmd(ch <-chan streamEvent) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return watcherStreamDoneMsg{}
		}
		if ev.done {
			return watcherStreamDoneMsg{err: ev.err}
		}
		return watcherStreamChunkMsg{text: ev.text, ch: ch}
	}
}
