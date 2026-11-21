package tunnel

import (
	crand "crypto/rand"
	"crypto/tls"
	"encoding/base32"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"time"

	"golang.org/x/net/http2"
)

// dial opens a connection to the target. For TLS targets it negotiates ALPN h2
// and requires the server to actually select it — older servers that clear ALPN
// predate the standard-h2 control traffic and are rejected.
func dial(d *net.Dialer, t target, tlsConfig *tls.Config) (net.Conn, error) {
	if t.plaintext {
		conn, err := d.Dial("tcp", t.key())
		if err != nil {
			return nil, err
		}
		return conn, nil
	}

	cfg := tlsConfig.Clone()
	cfg.ServerName = t.servername
	cfg.NextProtos = []string{"h2"}

	td := &tls.Dialer{NetDialer: d, Config: cfg}
	conn, err := td.Dial("tcp", t.key())
	if err != nil {
		return nil, err
	}
	tlsConn := conn.(*tls.Conn)
	if proto := tlsConn.ConnectionState().NegotiatedProtocol; proto != "h2" {
		_ = tlsConn.Close()
		return nil, fmt.Errorf("tunnel server did not negotiate h2 ALPN (got %q)", proto)
	}
	return tlsConn, nil
}

// serve runs the role-flipped HTTP/2 server over the dialed connection: we dialed
// out as a client, but Restate Cloud drives the connection as the HTTP/2 client,
// so we serve. It blocks until the connection is torn down.
//
// ReadIdleTimeout + PingTimeout are the server-initiated liveness watchdog: after
// the connection is read-idle for pingInterval the server sends an HTTP/2 PING,
// and if it isn't acked within pingTimeout the connection is closed (which makes
// ServeConn return and the slot reconnect) — the Go-native equivalent of the TS
// SDK's ping watchdog.
func (c *connection) serve() {
	srv := &http2.Server{
		MaxConcurrentStreams: c.maxConcurrentStreams,
		ReadIdleTimeout:      c.pingInterval,
		PingTimeout:          c.pingTimeout,
	}
	srv.ServeConn(c.netConn, &http2.ServeConnOpts{Handler: c})
}

// crockfordEnc is Crockford base32 (no padding) — 16 bytes encode to 26 chars, a
// ULID-like, roughly time-sortable id.
var crockfordEnc = base32.NewEncoding("0123456789ABCDEFGHJKMNPQRSTVWXYZ").WithPadding(base32.NoPadding)

// newConnectionID returns a unique id per h2 connection attempt, for cross-side
// diagnostics: a 48-bit millisecond timestamp followed by 80 random bits.
func newConnectionID() string {
	var b [16]byte
	ms := uint64(time.Now().UnixMilli())
	b[0] = byte(ms >> 40)
	b[1] = byte(ms >> 32)
	b[2] = byte(ms >> 24)
	b[3] = byte(ms >> 16)
	b[4] = byte(ms >> 8)
	b[5] = byte(ms)
	_, _ = crand.Read(b[6:])
	return crockfordEnc.EncodeToString(b[:])
}

// defaultWorkerID returns a stable-ish per-process worker id: the sanitized
// hostname (from $HOSTNAME, else os.Hostname) plus a random suffix so replicas
// that share a hostname still differ.
func defaultWorkerID() string {
	host := os.Getenv(envHostname)
	if host == "" {
		host, _ = os.Hostname()
	}
	host = sanitizeHeaderValue(host)
	if host == "" {
		host = "sdk-go"
	}
	var r [4]byte
	_, _ = crand.Read(r[:])
	return host + "-" + hex.EncodeToString(r[:])
}

// sanitizeHeaderValue drops bytes that aren't safe in an HTTP header value,
// keeping printable ASCII except space.
func sanitizeHeaderValue(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] > 0x20 && s[i] < 0x7f {
			out = append(out, s[i])
		}
	}
	return string(out)
}
