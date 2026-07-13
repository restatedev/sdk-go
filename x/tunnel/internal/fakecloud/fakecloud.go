// Package fakecloud is a minimal fake of the Restate Cloud tunnel server for
// tests. It accepts inbound connections and — like the real thing — runs the
// HTTP/2 *client* side over the accepted socket (the role flip): it opens
// GET /_/start-tunnel with the request body held open, reads the deployment's
// credential response headers, then completes the handshake by sending request
// trailers decided by the test. Afterwards the test can open further streams on
// the same session to model forwarded invocations.
package fakecloud

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"sync"

	"golang.org/x/net/http2"
)

// DecideFunc returns the handshake trailers for the index-th connection, or nil
// to stall (never send trailers).
type DecideFunc func(index int) map[string]string

// Cloud is a running fake tunnel server.
type Cloud struct {
	Addr string

	ln     net.Listener
	tr     *http2.Transport
	decide DecideFunc

	mu      sync.Mutex
	conns   []*Conn
	waiters map[int][]chan *Conn
}

// Conn is one accepted tunnel connection, exposing the role-flipped h2 client
// session so the test can open forwarded-invocation streams.
type Conn struct {
	Index int
	Creds http.Header // the deployment's /_/start-tunnel response headers

	cc  *http2.ClientConn
	raw net.Conn
	pw  *io.PipeWriter
}

// Start launches a fake cloud on 127.0.0.1. If tlsConf is nil it serves
// plaintext h2 (prior knowledge); otherwise it serves TLS (advertise h2 in
// tlsConf.NextProtos).
func Start(tlsConf *tls.Config, decide DecideFunc) (*Cloud, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	if tlsConf != nil {
		ln = tls.NewListener(ln, tlsConf)
	}

	c := &Cloud{
		Addr:    ln.Addr().String(),
		ln:      ln,
		tr:      &http2.Transport{AllowHTTP: true},
		decide:  decide,
		waiters: map[int][]chan *Conn{},
	}
	go c.acceptLoop()
	return c, nil
}

func (c *Cloud) acceptLoop() {
	for {
		raw, err := c.ln.Accept()
		if err != nil {
			return
		}
		go c.handle(raw)
	}
}

func (c *Cloud) handle(raw net.Conn) {
	cc, err := c.tr.NewClientConn(raw)
	if err != nil {
		_ = raw.Close()
		return
	}

	c.mu.Lock()
	idx := len(c.conns)
	conn := &Conn{Index: idx, cc: cc, raw: raw}
	c.conns = append(c.conns, conn)
	waiters := c.waiters[idx]
	delete(c.waiters, idx)
	c.mu.Unlock()

	for _, w := range waiters {
		w <- conn
	}

	c.handshake(conn)
}

func (c *Cloud) handshake(conn *Conn) {
	trailers := c.decide(conn.Index)

	pr, pw := io.Pipe()
	conn.pw = pw

	req, _ := http.NewRequest(http.MethodGet, "https://tunnel.test/_/start-tunnel", pr)
	// Pre-declare the trailer keys so the h2 client both announces them and sends
	// their values when the body ends. Values are fixed before RoundTrip, so
	// there is no concurrent access to req.Trailer.
	req.Trailer = http.Header{}
	for k, v := range trailers {
		req.Trailer.Set(k, v)
	}

	resp, err := conn.cc.RoundTrip(req)
	if err != nil {
		_ = pw.Close()
		return
	}
	conn.Creds = resp.Header.Clone()

	if trailers == nil {
		// Stall: leave the request body open so the deployment's handshake times
		// out (models a hung server / never-authorized tunnel).
		return
	}

	// End the request body, which flushes the trailers, completing the handshake.
	_ = pw.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
}

// Response is a collected response from a forwarded stream.
type Response struct {
	Status int
	Header http.Header
	Body   []byte
}

// Roundtrip opens a new stream on the connection modeling a forwarded request,
// and collects the full response.
func (conn *Conn) Roundtrip(method, path string, header http.Header, body []byte) (*Response, error) {
	var r io.Reader
	if body != nil {
		r = &readerNopCloser{data: body}
	}
	req, err := http.NewRequest(method, "https://tunnel.test"+path, r)
	if err != nil {
		return nil, err
	}
	for k, vs := range header {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}

	resp, err := conn.cc.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return &Response{Status: resp.StatusCode, Header: resp.Header, Body: data}, nil
}

// WaitForConnection blocks until the index-th connection has arrived.
func (c *Cloud) WaitForConnection(ctx context.Context, index int) (*Conn, error) {
	c.mu.Lock()
	if index < len(c.conns) {
		conn := c.conns[index]
		c.mu.Unlock()
		return conn, nil
	}
	ch := make(chan *Conn, 1)
	c.waiters[index] = append(c.waiters[index], ch)
	c.mu.Unlock()

	select {
	case conn := <-ch:
		return conn, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// ConnectionCount returns how many connections have arrived so far.
func (c *Cloud) ConnectionCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.conns)
}

// Close shuts the fake cloud down and tears down all sessions.
func (c *Cloud) Close() error {
	err := c.ln.Close()
	c.mu.Lock()
	conns := append([]*Conn(nil), c.conns...)
	c.mu.Unlock()
	for _, conn := range conns {
		if conn.pw != nil {
			_ = conn.pw.Close()
		}
		_ = conn.cc.Close()
		_ = conn.raw.Close()
	}
	return err
}

type readerNopCloser struct {
	data []byte
	off  int
}

func (r *readerNopCloser) Read(p []byte) (int, error) {
	if r.off >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.off:])
	r.off += n
	return n, nil
}
