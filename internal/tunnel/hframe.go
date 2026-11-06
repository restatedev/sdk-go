package tunnel

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"
)

// HTTP/2 frame types and flags we care about (RFC 7540).
const (
	frameData         = 0x0
	frameHeaders      = 0x1
	frameContinuation = 0x9

	flagEndStream  = 0x1
	flagEndHeaders = 0x4
	flagPadded     = 0x8
	flagPriority   = 0x20
)

// clientPreface is the 24-byte HTTP/2 client connection preface.
const clientPreface = "PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n"

// The tunnel server completes the handshake by sending these fields as HTTP/2
// request trailers. Go's net/http (and x/net/http2) server only surfaces
// request trailers to the handler if they were announced up-front via a
// "Trailer" header — but the tunnel server decides them late and sends them
// unannounced. We therefore transparently inject the announcement ourselves (see
// injectTrailerAnnouncement).
const trailerAnnounceName = "trailer"
const trailerAnnounceValue = "tunnel-status,tunnel-name,proxy-url,tunnel-url"

// trailerAnnounceField is the HPACK "Literal Header Field Never Indexed — New
// Name" (RFC 7541 §6.2.3) encoding of "trailer: <keys>". Never-indexed means it
// is NOT added to the HPACK dynamic table, so injecting it does not desync the
// table shared across the connection — every subsequent HEADERS frame from the
// server still decodes correctly.
func trailerAnnounceField() []byte {
	// Both name (7) and value (45) are < 127, so their lengths fit the 7-bit
	// string-length prefix without a continuation byte, and we send them
	// un-Huffman'd (H bit clear).
	b := make([]byte, 0, 2+len(trailerAnnounceName)+1+len(trailerAnnounceValue))
	b = append(b, 0x10) // never-indexed, name index 0 (new name follows)
	b = append(b, byte(len(trailerAnnounceName)))
	b = append(b, trailerAnnounceName...)
	b = append(b, byte(len(trailerAnnounceValue)))
	b = append(b, trailerAnnounceValue...)
	return b
}

// prefixConn is a net.Conn that first replays a buffer of already-read bytes,
// then reads through to the underlying connection.
type prefixConn struct {
	net.Conn
	prefix []byte
	off    int
}

func (c *prefixConn) Read(p []byte) (int, error) {
	if c.off < len(c.prefix) {
		n := copy(p, c.prefix[c.off:])
		c.off += n
		return n, nil
	}
	return c.Conn.Read(p)
}

// injectTrailerAnnouncement reads the start of the (role-flipped) HTTP/2
// connection — the client preface and any frames up to and including the first
// request's complete header block — appends a "Trailer" announcement to that
// header block, and returns a net.Conn that replays the (rewritten) prefix and
// then reads through unchanged. This makes the SDK's HTTP/2 server surface the
// handshake trailers to the handler.
//
// It only rewrites the single first header block; everything after is passed
// through verbatim. A read deadline bounds the wait for the peer to open its
// first stream.
func injectTrailerAnnouncement(conn net.Conn, timeout time.Duration) (net.Conn, error) {
	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	defer conn.SetReadDeadline(time.Time{})

	br := bufio.NewReader(conn)

	var out bytes.Buffer

	preface := make([]byte, len(clientPreface))
	if _, err := io.ReadFull(br, preface); err != nil {
		return nil, fmt.Errorf("read h2 preface: %w", err)
	}
	if string(preface) != clientPreface {
		return nil, fmt.Errorf("unexpected h2 client preface")
	}
	out.Write(preface)

	inject := trailerAnnounceField()
	var pendingHeaderStream uint32 // non-zero while inside a header block awaiting CONTINUATION with END_HEADERS
	injected := false

	for !injected {
		hdr := make([]byte, 9)
		if _, err := io.ReadFull(br, hdr); err != nil {
			return nil, fmt.Errorf("read h2 frame header: %w", err)
		}
		length := int(hdr[0])<<16 | int(hdr[1])<<8 | int(hdr[2])
		ftype := hdr[3]
		flags := hdr[4]
		streamID := binary.BigEndian.Uint32(hdr[5:9]) & 0x7fffffff

		payload := make([]byte, length)
		if _, err := io.ReadFull(br, payload); err != nil {
			return nil, fmt.Errorf("read h2 frame payload: %w", err)
		}

		endHeaders := flags&flagEndHeaders != 0
		terminal := false // does this frame complete the first header block?
		switch {
		case pendingHeaderStream == 0 && ftype == frameHeaders:
			if endHeaders {
				terminal = true
			} else {
				pendingHeaderStream = streamID
			}
		case pendingHeaderStream != 0 && ftype == frameContinuation && streamID == pendingHeaderStream:
			if endHeaders {
				terminal = true
				pendingHeaderStream = 0
			}
		case pendingHeaderStream == 0 && ftype == frameData:
			// A DATA frame before any header block would be a protocol error from
			// the peer; bail out rather than guessing.
			return nil, fmt.Errorf("unexpected DATA frame before first header block")
		}

		if !terminal {
			out.Write(hdr)
			out.Write(payload)
			continue
		}

		newPayload, err := appendToHeaderBlock(ftype, flags, payload, inject)
		if err != nil {
			return nil, err
		}
		writeFrameHeader(&out, len(newPayload), ftype, flags, streamID)
		out.Write(newPayload)
		injected = true
	}

	// Drain anything bufio already buffered past the injected block into the prefix.
	if n := br.Buffered(); n > 0 {
		buf := make([]byte, n)
		if _, err := io.ReadFull(br, buf); err != nil {
			return nil, err
		}
		out.Write(buf)
	}

	return &prefixConn{Conn: conn, prefix: out.Bytes()}, nil
}

// appendToHeaderBlock inserts the encoded field at the end of a header block
// fragment, before any padding. CONTINUATION frames carry no padding or priority,
// so their whole payload is fragment; HEADERS frames may be preceded by a pad
// length and priority bytes and followed by padding.
func appendToHeaderBlock(ftype, flags byte, payload, field []byte) ([]byte, error) {
	if ftype == frameContinuation {
		return append(append([]byte{}, payload...), field...), nil
	}

	// HEADERS frame: strip trailing padding, append, then keep padding after.
	padLen := 0
	if flags&flagPadded != 0 {
		if len(payload) < 1 {
			return nil, fmt.Errorf("padded HEADERS frame too short")
		}
		padLen = int(payload[0])
	}
	fragEnd := len(payload) - padLen
	if fragEnd < 0 {
		return nil, fmt.Errorf("HEADERS frame padding exceeds payload")
	}
	out := make([]byte, 0, len(payload)+len(field))
	out = append(out, payload[:fragEnd]...)
	out = append(out, field...)
	out = append(out, payload[fragEnd:]...)
	return out, nil
}

func writeFrameHeader(w *bytes.Buffer, length int, ftype, flags byte, streamID uint32) {
	var hdr [9]byte
	hdr[0] = byte(length >> 16)
	hdr[1] = byte(length >> 8)
	hdr[2] = byte(length)
	hdr[3] = ftype
	hdr[4] = flags
	binary.BigEndian.PutUint32(hdr[5:9], streamID&0x7fffffff)
	w.Write(hdr[:])
}
