package tunnel

import (
	"bytes"
	"io"
	"net"
	"testing"
	"time"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
)

// memConn is a net.Conn backed by an in-memory reader (writes are discarded).
type memConn struct {
	r io.Reader
}

func (c *memConn) Read(p []byte) (int, error)       { return c.r.Read(p) }
func (c *memConn) Write(p []byte) (int, error)      { return len(p), nil }
func (c *memConn) Close() error                     { return nil }
func (c *memConn) LocalAddr() net.Addr              { return dummyAddr{} }
func (c *memConn) RemoteAddr() net.Addr             { return dummyAddr{} }
func (c *memConn) SetDeadline(time.Time) error      { return nil }
func (c *memConn) SetReadDeadline(time.Time) error  { return nil }
func (c *memConn) SetWriteDeadline(time.Time) error { return nil }

type dummyAddr struct{}

func (dummyAddr) Network() string { return "mem" }
func (dummyAddr) String() string  { return "mem" }

// TestInjectTrailerAnnouncement proves the injector adds a "trailer"
// announcement to a header block that carried none — the exact condition that
// lets the SDK's HTTP/2 server surface the tunnel server's unannounced handshake
// trailers.
func TestInjectTrailerAnnouncement(t *testing.T) {
	// Build a client preface + a HEADERS frame for /_/start-tunnel with NO trailer
	// announcement.
	var raw bytes.Buffer
	raw.WriteString(clientPreface)

	var hbuf bytes.Buffer
	enc := hpack.NewEncoder(&hbuf)
	for _, f := range []hpack.HeaderField{
		{Name: ":method", Value: "GET"},
		{Name: ":path", Value: "/_/start-tunnel"},
		{Name: ":scheme", Value: "https"},
		{Name: ":authority", Value: "tunnel.test"},
	} {
		if err := enc.WriteField(f); err != nil {
			t.Fatal(err)
		}
	}

	fr := http2.NewFramer(&raw, nil)
	if err := fr.WriteHeaders(http2.HeadersFrameParam{
		StreamID:      1,
		BlockFragment: hbuf.Bytes(),
		EndHeaders:    true,
		EndStream:     false,
	}); err != nil {
		t.Fatal(err)
	}

	conn, err := injectTrailerAnnouncement(&memConn{r: bytes.NewReader(raw.Bytes())}, time.Second)
	if err != nil {
		t.Fatalf("injectTrailerAnnouncement: %v", err)
	}
	out, err := io.ReadAll(conn)
	if err != nil {
		t.Fatal(err)
	}

	// Skip the preface, then decode the (rewritten) HEADERS frame.
	if !bytes.HasPrefix(out, []byte(clientPreface)) {
		t.Fatal("output missing client preface")
	}
	rfr := http2.NewFramer(nil, bytes.NewReader(out[len(clientPreface):]))
	rfr.ReadMetaHeaders = hpack.NewDecoder(4096, nil)

	f, err := rfr.ReadFrame()
	if err != nil {
		t.Fatalf("read rewritten frame: %v", err)
	}
	mh, ok := f.(*http2.MetaHeadersFrame)
	if !ok {
		t.Fatalf("expected MetaHeadersFrame, got %T", f)
	}

	var trailer string
	for _, hf := range mh.Fields {
		if hf.Name == "trailer" {
			trailer = hf.Value
		}
	}
	if trailer != trailerAnnounceValue {
		t.Fatalf("trailer announcement = %q, want %q", trailer, trailerAnnounceValue)
	}
}
