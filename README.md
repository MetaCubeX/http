# HTTP
[![Go Reference](https://pkg.go.dev/badge/github.com/metacubex/http.svg)](https://pkg.go.dev/github.com/metacubex/http)

backport net/http for go1.20+

export `net/http` from:
https://github.com/golang/go/tree/a70addd3b31ccb685f48867e24c6c2b4dc364a11/src/net/http

special modify:
- add TLSConn interface and not depend on *tls.Conn type
    ```Go
    type TLSConn interface {
        net.Conn
        NetConn() net.Conn
        HandshakeContext(ctx context.Context) error
        ConnectionState() tls.ConnectionState
    }
    ```
- add CloseHttp2Connections interface to close all http2 underlay connections
    ```Go
    type CloseHttp2Connections interface {
        CloseHttp2Connections()
    }
    ```
- net/http: fix NewClientConn not fully respected the passed-in context | https://go-review.googlesource.com/c/go/+/775408
- net/http/internal/http2: support non-tls.Conn TLS conn in forceCloseConn | https://go-review.googlesource.com/c/go/+/775409
- net/http: fix Server's UnencryptedHTTP2 does not work TLS without ALPN | https://go-review.googlesource.com/c/go/+/775734
- net/http: fix Transport's UnencryptedHTTP2 does not work TLS without ALPN | https://go-review.googlesource.com/c/go/+/776240