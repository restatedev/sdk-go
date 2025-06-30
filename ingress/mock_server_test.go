package ingress_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nsf/jsondiff"
	"github.com/stretchr/testify/require"

	"github.com/restatedev/sdk-go/internal/ingress"
)

type mockIngressServer struct {
	URL     string
	s       *httptest.Server
	method  string
	path    string
	headers map[string]string
	body    []byte
	query   map[string]string
}

func newMockIngressServer() *mockIngressServer {
	m := &mockIngressServer{}
	m.s = httptest.NewServer(m)
	m.URL = m.s.URL
	return m
}

func (m *mockIngressServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.method = r.Method
	m.path = r.URL.Path
	m.body, _ = io.ReadAll(r.Body)

	m.headers = make(map[string]string)
	for k, v := range r.Header {
		m.headers[k] = v[0]
	}

	m.query = make(map[string]string)
	for k, v := range r.URL.Query() {
		m.query[k] = v[0]
	}

	if strings.HasSuffix(m.path, "/send") {
		inv := ingress.Invocation{
			Id:     "inv_1",
			Status: "Accepted",
		}
		json.NewEncoder(w).Encode(&inv)
	} else {
		w.Write([]byte("OK"))
	}
}

func (m *mockIngressServer) AssertPath(t *testing.T, expectedPath string) {
	require.Equalf(t, "/"+expectedPath, m.path, "expected path %s, got %s", expectedPath, m.path)
}

func (m *mockIngressServer) AssertMethod(t *testing.T, expectedMethod string) {
	require.Equalf(t, expectedMethod, m.method, "expected method %s, got %s", expectedMethod, m.method)
}

func (m *mockIngressServer) AssertContentType(t *testing.T, contentType string) {
	require.NotNil(t, m.headers)
	require.Equal(t, contentType, m.headers["Content-Type"])
}

func (m *mockIngressServer) AssertNoContentType(t *testing.T) {
	require.NotNil(t, m.headers)
	require.Empty(t, m.headers["Content-Type"])
}

func (m *mockIngressServer) AssertNoBody(t *testing.T) {
	require.NotNil(t, m.headers)
	require.Empty(t, m.body)
}

func (m *mockIngressServer) AssertHeaders(t *testing.T, expectedHeaders map[string]string) {
	if expectedHeaders == nil && m.headers == nil {
		return
	}
	if expectedHeaders != nil && m.headers == nil {
		require.Fail(t, "expected headers but got none")
	}
	if expectedHeaders == nil && m.headers != nil {
		require.Fail(t, "expected no headers but got some")
	}
	reqHeaders := make(map[string]string)
	for k, v := range m.headers {
		reqHeaders[strings.ToLower(k)] = v
	}
	for k, v := range expectedHeaders {
		h, ok := reqHeaders[strings.ToLower(k)]
		require.Truef(t, ok, "header %s not found in request", k)
		require.Equalf(t, v, h, "header %s not equal to expected value", k)
	}
}

func (m *mockIngressServer) AssertBody(t *testing.T, expectedBody []byte) {
	if len(expectedBody) == 0 && len(m.body) == 0 {
		return
	}
	require.Equalf(t, len(expectedBody), len(m.body), "expected body length %d, got %d", len(expectedBody), len(m.body))

	diff, _ := jsondiff.Compare(expectedBody, m.body, &jsondiff.Options{})
	require.Equalf(t, diff, jsondiff.FullMatch, "expected body %s, got %s; diff: %s", string(m.body), string(expectedBody), diff.String())
}

func (m *mockIngressServer) AssertQuery(t *testing.T, expectedQuery map[string]string) {
	if expectedQuery == nil && m.query == nil {
		return
	}
	if expectedQuery != nil && len(expectedQuery) > 0 && m.query == nil {
		require.Fail(t, "expected query but got none")
	}
	if expectedQuery == nil && m.query != nil && len(m.query) > 0 {
		require.Fail(t, "expected no query but got some")
	}
	for k, v := range expectedQuery {
		h, ok := m.query[k]
		require.Truef(t, ok, "query %s not found in request", k)
		require.Equalf(t, v, h, "query %s not equal to expected value", k)
	}
}

func (m *mockIngressServer) Close() {
	m.s.Close()
}
