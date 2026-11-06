package tunnel

import "strings"

// forwardedTail strips the tunnel's forwarded prefix "/<scheme>/<host>/<port>"
// and returns the tail — the path the SDK handler should see — plus true.
//
// A forwarded invocation arrives down the tunnel with its destination encoded in
// the path (e.g. "/http/in-process/9080/invoke/Svc/handler"); the cloud proxy has
// already stripped the "/<env>/<tunnel>" rendezvous prefix. For an in-process SDK
// deployment the scheme/host/port are vestigial (the receiver *is* the service),
// so we drop exactly those three segments and keep the tail ("/discover",
// "/invoke/<svc>/<handler>", …).
//
// The tail is passed through without re-encoding: the SDK verifies each request's
// identity JWT against the signed service-relative path, so re-encoding,
// normalization or case-folding of the tail would break the match. Any query
// string is preserved (it is not part of the JWT audience).
//
// Returns ("", false) if the path isn't a forwarded "/<scheme>/<host>/<port>/..."
// path.
func forwardedTail(rawURL string) (string, bool) {
	path := rawURL
	query := ""
	if i := strings.IndexByte(rawURL, '?'); i != -1 {
		path, query = rawURL[:i], rawURL[i:]
	}

	// seg = ["", scheme, host, port, ...tail]
	seg := strings.Split(path, "/")
	// The port segment must be numeric — that's what distinguishes a real
	// forwarded prefix from an unprefixed SDK path that happens to have three
	// segments (e.g. "/invoke/Svc/handler" must NOT parse as scheme=invoke,
	// host=Svc, port=handler and dispatch "/" to the SDK).
	if len(seg) < 4 || seg[1] == "" || seg[2] == "" || !isNumeric(seg[3]) {
		return "", false
	}

	return "/" + strings.Join(seg[4:], "/") + query, true
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
