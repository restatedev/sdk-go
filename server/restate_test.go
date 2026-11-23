package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	restate "github.com/restatedev/sdk-go"
	"github.com/stretchr/testify/require"
)

func TestDiscoveryPreemptable(t *testing.T) {
	echo := func(ctx restate.Context, in string) (string, error) { return in, nil }
	srv := NewRestate().
		Bind(restate.NewService("NonPreemptable", restate.WithPreemptable(false)).
			Handler("overridden", restate.NewServiceHandler(echo, restate.WithPreemptable(true))).
			Handler("inherited", restate.NewServiceHandler(echo))).
		Bind(restate.NewService("Default").
			Handler("unset", restate.NewServiceHandler(echo)))
	handler, err := srv.Handler()
	require.NoError(t, err)

	tests := []struct {
		name        string
		accept      string
		contentType string
		// "preemptable" values found in the manifest, keyed by "service" or "service/handler".
		// Services and handlers without the field are absent from the map.
		want map[string]bool
	}{
		{
			name:        "v6 negotiated and preemptable emitted only where set",
			accept:      "application/vnd.restate.endpointmanifest.v4+json, application/vnd.restate.endpointmanifest.v6+json",
			contentType: "application/vnd.restate.endpointmanifest.v6+json",
			want:        map[string]bool{"NonPreemptable": false, "NonPreemptable/overridden": true},
		},
		{
			name:        "preemptable dropped below v6",
			accept:      "application/vnd.restate.endpointmanifest.v4+json",
			contentType: "application/vnd.restate.endpointmanifest.v4+json",
			want:        map[string]bool{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/discover", nil)
			req.Header.Set("accept", tt.accept)
			rec := httptest.NewRecorder()
			handler(rec, req)
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			require.Equal(t, tt.contentType, rec.Header().Get("Content-Type"))

			var manifest struct {
				Services []struct {
					Name        string `json:"name"`
					Preemptable *bool  `json:"preemptable"`
					Handlers    []struct {
						Name        string `json:"name"`
						Preemptable *bool  `json:"preemptable"`
					} `json:"handlers"`
				} `json:"services"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &manifest))
			require.Len(t, manifest.Services, 2)

			got := map[string]bool{}
			for _, s := range manifest.Services {
				if s.Preemptable != nil {
					got[s.Name] = *s.Preemptable
				}
				for _, h := range s.Handlers {
					if h.Preemptable != nil {
						got[s.Name+"/"+h.Name] = *h.Preemptable
					}
				}
			}
			require.Equal(t, tt.want, got)
		})
	}
}
