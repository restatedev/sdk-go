package restate

import (
	"maps"
	"slices"
	"testing"

	"github.com/restatedev/sdk-go/internal/options"
	"github.com/restatedev/sdk-go/internal/stringmap"
	"github.com/stretchr/testify/require"
)

func TestWithHeaders(t *testing.T) {
	for _, tt := range []struct {
		name    string
		headers map[string]string
		keys    []string
	}{
		{name: "nil"},
		{name: "empty", headers: map[string]string{}},
		{name: "single", headers: map[string]string{"request-id": "request-123"}, keys: []string{"request-id"}},
		{
			name: "multiple",
			headers: map[string]string{
				"request-id":  "request-123",
				"traceparent": "00-trace-span-01",
				"X-Metadata":  "example-metadata",
				"x-metadata":  "",
			},
			keys: []string{"X-Metadata", "request-id", "traceparent", "x-metadata"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// Rebuild equivalent maps in both insertion orders and iterate each
			// repeatedly: insertion order alone does not control Go map iteration.
			for _, reverse := range []bool{false, true} {
				keys := slices.Clone(tt.keys)
				if reverse {
					slices.Reverse(keys)
				}
				var headers map[string]string
				if tt.headers != nil {
					headers = make(map[string]string, len(keys))
				}
				for _, key := range keys {
					headers[key] = tt.headers[key]
				}
				before := maps.Clone(headers)
				opt := WithHeaders(headers)
				var request options.RequestOptions
				var send options.SendOptions
				var ingressRequest options.IngressRequestOptions
				var ingressSend options.IngressSendOptions
				opt.BeforeRequest(&request)
				opt.BeforeSend(&send)
				opt.BeforeIngressRequest(&ingressRequest)
				opt.BeforeIngressSend(&ingressSend)
				for name, got := range map[string]stringmap.Map{
					"request":         request.Headers,
					"send":            send.Headers,
					"ingress request": ingressRequest.Headers,
					"ingress send":    ingressSend.Headers,
				} {
					t.Run(name, func(t *testing.T) {
						require.NotNil(t, got)
						for range 100 {
							var gotKeys []string
							for key, value := range got.Iter() {
								gotKeys = append(gotKeys, key)
								require.Equal(t, headers[key], value, "header %q", key)
							}
							require.Equal(t, tt.keys, gotKeys)
						}
					})
				}
				require.Equal(t, before, headers, "must not modify caller's map")
			}
		})
	}
}
