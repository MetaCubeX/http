package http_test

import (
	"context"
	"io"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	. "github.com/metacubex/http"
)

// trackedConn is the raw connection the transport dials. Its finalizer runs
// once the HTTP/2 client connection that owns it is unreachable.
type trackedConn struct {
	net.Conn
}

// A closed HTTP/2 client connection must not stay reachable from its own
// timers. The read loop's health-check timer (SendPingTimeout) was reset after
// every frame but never stopped, and forgetStreamID re-armed the idle timer
// (IdleConnTimeout) on a connection the read loop had already closed. Both
// callbacks hold the *ClientConn, and with it the framer, buffers and the
// underlying net.Conn, until they fire.
func TestTransportClosedHTTP2ConnNotHeldByTimers(t *testing.T) {
	run(t, testTransportClosedHTTP2ConnNotHeldByTimers, []testMode{http2Mode})
}

func testTransportClosedHTTP2ConnNotHeldByTimers(t *testing.T, mode testMode) {
	// The timers are set long enough that a connection they hold is still
	// held when the test gives up.
	const timerPeriod = time.Minute
	for _, tc := range []struct {
		name string
		// ping sets HTTP2Config.SendPingTimeout; idle sets Transport.IdleConnTimeout.
		ping, idle bool
		// cutUnderStream closes the connection while a response body is still open,
		// so the stream is forgotten after the read loop has closed the connection.
		// Otherwise each response is read to the end and the idle connection is
		// closed by CloseIdleConnections.
		cutUnderStream bool
	}{
		{name: "no-timers/closed-idle"},
		{name: "no-timers/cut-under-stream", cutUnderStream: true},
		{name: "ping/closed-idle", ping: true},
		{name: "ping/cut-under-stream", ping: true, cutUnderStream: true},
		{name: "idle/cut-under-stream", idle: true, cutUnderStream: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			release := make(chan struct{})
			cst := newClientServerTest(t, mode, HandlerFunc(func(w ResponseWriter, r *Request) {
				w.WriteHeader(200)
				w.(Flusher).Flush()
				if r.URL.Path == "/stream" {
					select {
					case <-r.Context().Done():
					case <-release:
					}
				}
			}))
			defer close(release)

			var live atomic.Int64
			var mu sync.Mutex
			var last *trackedConn
			tr := cst.tr
			if tc.idle {
				tr.IdleConnTimeout = timerPeriod
			}
			if tc.ping {
				tr.HTTP2 = &HTTP2Config{SendPingTimeout: timerPeriod}
			}
			dial := tr.DialContext
			if dial == nil {
				dial = (&net.Dialer{}).DialContext
			}
			tr.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
				raw, err := dial(ctx, network, addr)
				if err != nil {
					return nil, err
				}
				c := &trackedConn{Conn: raw}
				live.Add(1)
				runtime.SetFinalizer(c, func(*trackedConn) { live.Add(-1) })
				mu.Lock()
				last = c
				mu.Unlock()
				return c, nil
			}

			const conns = 16
			for i := 0; i < conns; i++ {
				path := "/"
				if tc.cutUnderStream {
					path = "/stream"
				}
				res, err := cst.c.Get(cst.ts.URL + path)
				if err != nil {
					t.Fatalf("request %d: %v", i, err)
				}
				if tc.cutUnderStream {
					mu.Lock()
					c := last
					last = nil
					mu.Unlock()
					_ = c.Conn.Close()
					_, _ = io.Copy(io.Discard, res.Body)
				} else {
					_, _ = io.Copy(io.Discard, res.Body)
				}
				_ = res.Body.Close()
				tr.CloseIdleConnections()
			}
			mu.Lock()
			last = nil
			mu.Unlock()

			// A stopped timer keeps its callback until the runtime drops it from
			// its heap, which it does lazily; allow a small tail.
			const allowed = conns / 8
			deadline := time.Now().Add(3 * time.Second)
			for live.Load() > allowed && time.Now().Before(deadline) {
				runtime.GC()
				time.Sleep(20 * time.Millisecond)
			}
			if left := live.Load(); left > allowed {
				t.Fatalf("%d of %d closed connections still reachable 3s after close (allowed %d)", left, conns, allowed)
			}
		})
	}
}
