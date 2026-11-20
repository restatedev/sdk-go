package ingress_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	restate "github.com/restatedev/sdk-go"
	"github.com/restatedev/sdk-go/ingress"
	"github.com/stretchr/testify/require"
)

func TestIngressPreservesKeyPathSegments(t *testing.T) {
	keys := []struct{ key, escaped string }{
		{"simple-key", "simple-key"},
		{"folder/item", "folder%2Fitem"},
		{"question?x=1#fragment", "question%3Fx=1%23fragment"},
		{"literal%2Fpercent%", "literal%252Fpercent%25"},
		{"雪 key", "%E9%9B%AA%20key"},
	}
	for _, key := range keys {
		for _, scoped := range []bool{false, true} {
			for _, send := range []bool{false, true} {
				for _, workflow := range []bool{false, true} {
					name := key.key
					if scoped {
						name += "/scoped"
					}
					if send {
						name += "/send"
					}
					if workflow {
						name += "/workflow"
					}
					t.Run(name, func(t *testing.T) {
						requests := make(chan *http.Request, 1)
						server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							requests <- r
							w.Header().Set("Content-Type", "application/json")
							if send {
								_, _ = io.WriteString(w, `{"invocationId":"inv_1","status":"Accepted"}`)
							} else {
								_, _ = io.WriteString(w, `"done"`)
							}
						}))
						defer server.Close()
						client := ingress.NewClient(server.URL)
						var requester ingress.Requester[string, string]
						if workflow {
							if scoped {
								requester = ingress.Workflow[string, string](client, "Orders", key.key, "run", restate.WithScope("tenant-a"))
							} else {
								requester = ingress.Workflow[string, string](client, "Orders", key.key, "run")
							}
						} else {
							if scoped {
								requester = ingress.Object[string, string](client, "Orders", key.key, "run", restate.WithScope("tenant-a"))
							} else {
								requester = ingress.Object[string, string](client, "Orders", key.key, "run")
							}
						}
						ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
						defer cancel()
						if send {
							_, err := requester.Send(ctx, "input")
							require.NoError(t, err)
						} else {
							output, err := requester.Request(ctx, "input")
							require.NoError(t, err)
							require.Equal(t, "done", output)
						}
						request := <-requests
						prefix := "/Orders/"
						suffix := "/run"
						if scoped {
							verb := "call"
							if send {
								verb = "send"
							}
							prefix = "/restate/scope/tenant-a/" + verb + "/Orders/"
						} else if send {
							suffix += "/send"
						}
						require.Equal(t, prefix+key.escaped+suffix, request.URL.EscapedPath())
						require.Empty(t, request.URL.RawQuery)
						require.Equal(t, http.MethodPost, request.Method)
					})
				}
			}
		}
	}
}

func TestIngressPreservesObservationPathSegments(t *testing.T) {
	for _, op := range []string{"attach", "output"} {
		for _, kind := range []string{"workflow", "object", "service"} {
			t.Run(kind+"/"+op, func(t *testing.T) {
				requests := make(chan *http.Request, 1)
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					requests <- r
					w.Header().Set("Content-Type", "application/json")
					_, _ = io.WriteString(w, `"done"`)
				}))
				defer server.Close()
				client := ingress.NewClient(server.URL)
				key := "folder/item?x=1#part%"
				escaped := "folder%2Fitem%3Fx=1%23part%25"
				var handle ingress.InvocationHandle[string]
				var path string
				switch kind {
				case "workflow":
					handle = ingress.WorkflowHandle[string](client, "Orders", key)
					path = "/restate/workflow/Orders/" + escaped
				case "object":
					handle = ingress.ObjectInvocationByIdempotencyKey[string](client, "Orders", key, "run", "request-key")
					path = "/restate/invocation/Orders/" + escaped + "/run/request-key"
				default:
					handle = ingress.ServiceInvocationByIdempotencyKey[string](client, "Orders", "run", "request-key")
					path = "/restate/invocation/Orders/run/request-key"
				}
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				var output string
				var err error
				if op == "attach" {
					output, err = handle.Attach(ctx)
				} else {
					output, err = handle.Output(ctx)
				}
				require.NoError(t, err)
				require.Equal(t, "done", output)
				request := <-requests
				require.Equal(t, path+"/"+op, request.URL.EscapedPath())
				require.Empty(t, request.URL.RawQuery)
				require.False(t, strings.Contains(request.RequestURI, "#"))
				require.Equal(t, http.MethodGet, request.Method)
			})
		}
	}
}
