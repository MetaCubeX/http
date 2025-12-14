// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package http_test

import (
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/metacubex/http"
)

func TestTransportNewClientConnRoundTrip(t *testing.T) { run(t, testTransportNewClientConnRoundTrip) }
func testTransportNewClientConnRoundTrip(t *testing.T, mode testMode) {
	cst := newClientServerTest(t, mode, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		io.WriteString(w, req.Host)
	}), optFakeNet)

	scheme := mode.Scheme() // http or https
	cc, err := cst.tr.NewClientConn(context.Background(), scheme, cst.ts.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer cc.Close()

	// Send requests for a couple different domains.
	// All use the same connection.
	for _, host := range []string{"example.tld", "go.dev"} {
		req, _ := http.NewRequest("GET", fmt.Sprintf("%v://%v/", scheme, host), nil)
		resp, err := cc.RoundTrip(req)
		if err != nil {
			t.Fatal(err)
		}
		got, _ := io.ReadAll(resp.Body)
		if string(got) != host {
			t.Errorf("got response body %q, want %v", got, host)
		}
		resp.Body.Close()

		// CloseIdleConnections does not close connections created by NewClientConn.
		cst.tr.CloseIdleConnections()
	}

	if err := cc.Err(); err != nil {
		t.Errorf("before close: ClientConn.Err() = %v, want nil", err)
	}

	cc.Close()
	if err := cc.Err(); err == nil {
		t.Errorf("after close: ClientConn.Err() = nil, want error")
	}

	req, _ := http.NewRequest("GET", scheme+"://example.tld/", nil)
	resp, err := cc.RoundTrip(req)
	if err == nil {
		resp.Body.Close()
		t.Errorf("after close: cc.RoundTrip succeeded, want error")
	}
	t.Log(err)
}
