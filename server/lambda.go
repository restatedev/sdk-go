package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/restatedev/sdk-go/internal/identity"
)

type LambdaRequest struct {
	Path            string            `json:"path"`
	RawPath         string            `json:"rawPath"`
	Body            string            `json:"body"`
	IsBase64Encoded bool              `json:"isBase64Encoded"`
	Headers         map[string]string `json:"headers"`
}

type LambdaResponse struct {
	StatusCode      int               `json:"statusCode"`
	Headers         map[string]string `json:"headers"`
	Body            string            `json:"body"`
	IsBase64Encoded bool              `json:"isBase64Encoded"`
}

type LambdaHandlerFunc func(ctx context.Context, event LambdaRequest) (LambdaResponse, error)

type lambdaResponseWriter struct {
	headers http.Header
	body    bytes.Buffer
	status  int
}

func (r *lambdaResponseWriter) Header() http.Header {
	return r.headers
}

func (r *lambdaResponseWriter) Write(body []byte) (int, error) {
	if r.status == -1 {
		r.status = http.StatusOK
	}

	// if the content type header is not set when we write the body we try to
	// detect one and set it by default. If the content type cannot be detected
	// it is automatically set to "application/octet-stream" by the
	// DetectContentType method
	if r.Header().Get("Content-Type") == "" {
		r.Header().Add("Content-Type", http.DetectContentType(body))
	}

	return (&r.body).Write(body)
}

func (r *lambdaResponseWriter) WriteHeader(statusCode int) {
	r.status = statusCode
}

func (r *lambdaResponseWriter) Flush() {}

func (r *lambdaResponseWriter) LambdaResponse() LambdaResponse {
	headers := make(map[string]string, len(r.headers))
	for k, v := range r.headers {
		if len(v) == 0 {
			continue
		}
		headers[k] = v[0]
	}

	return LambdaResponse{
		Headers:         headers,
		StatusCode:      r.status,
		IsBase64Encoded: true,
		Body:            base64.StdEncoding.EncodeToString(r.body.Bytes()),
	}
}

// LambdaHandler obtains a Lambda handler function representing the bound services
// .Bidirectional(false) will be set on your behalf as Lambda only supports request-response communication
func (r *Restate) LambdaHandler() (LambdaHandlerFunc, error) {
	r.Bidirectional(false)

	if r.keyIDs == nil {
		r.systemLog.Warn("Accepting requests without validating request signatures; Invoke must be restricted")
	} else {
		ks, err := identity.ParseKeySetV1(r.keyIDs)
		if err != nil {
			return nil, fmt.Errorf("invalid request identity keys: %w", err)
		}
		r.keySet = ks
		r.systemLog.Info("Validating requests using signing keys", "keys", r.keyIDs)
	}

	return LambdaHandlerFunc(r.lambdaHandler), nil
}

func (r *Restate) lambdaHandler(ctx context.Context, event LambdaRequest) (LambdaResponse, error) {
	var path string
	if event.Path != "" {
		path = event.Path
	} else if event.RawPath != "" {
		path = event.RawPath
	}

	var body io.Reader
	if event.Body != "" {
		if event.IsBase64Encoded {
			body = base64.NewDecoder(base64.StdEncoding, strings.NewReader(event.Body))
		} else {
			body = strings.NewReader(event.Body)
		}
	}

	// method is not read so just set POST as a default
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, path, body)
	if err != nil {
		return LambdaResponse{StatusCode: http.StatusBadGateway}, err
	}
	req.RequestURI = path
	for k, v := range event.Headers {
		req.Header.Add(k, v)
	}

	rw := lambdaResponseWriter{headers: make(http.Header, 2), status: -1}

	r.handler(&rw, req)

	return rw.LambdaResponse(), nil
}
