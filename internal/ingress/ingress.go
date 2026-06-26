package ingress

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	restate "github.com/restatedev/sdk-go"
	"github.com/restatedev/sdk-go/encoding"

	"github.com/restatedev/sdk-go/internal/options"
)

const (
	idempotencyKeyHeader = "idempotency-key"
	limitKeyHeader       = "x-restate-limit-key"
	delayQuery           = "delay"
)

// Client is an ingress client used to initiate Restate invocations outside a Restate context.
type Client struct {
	baseUri    string
	clientOpts options.IngressClientOptions
}

type IngressParams struct {
	Service string
	Handler string
	Key     string
}

type IngressAttachParams struct {
	ServiceName    string
	MethodName     string
	ObjectKey      string
	InvocationID   string
	IdempotencyKey string
	WorkflowID     string
}

func NewClient(baseUri string, opts options.IngressClientOptions) *Client {
	return &Client{
		baseUri:    baseUri,
		clientOpts: opts,
	}
}

func (c *Client) Request(ctx context.Context, params IngressParams, input, output any, reqOpts options.IngressRequestOptions) error {
	return c.do(ctx, http.MethodPost, makeIngressUrl(params, reqOpts.Scope, false), input, output,
		reqOpts.IdempotencyKey,
		reqOpts.Headers,
		0,
		reqOpts.LimitKey,
		reqOpts.InputCodec,
		reqOpts.OutputCodec)
}

func (c *Client) Send(ctx context.Context, params IngressParams, input any, sendOpts options.IngressSendOptions) (Invocation, error) {
	url := makeIngressUrl(params, sendOpts.Scope, true)
	var output Invocation
	err := c.do(ctx, http.MethodPost, url, input, &output, sendOpts.IdempotencyKey, sendOpts.Headers, sendOpts.Delay, sendOpts.LimitKey, sendOpts.Codec, encoding.JSONCodec)
	return output, err
}

func (c *Client) Attach(ctx context.Context, params IngressAttachParams, output any, attachOpts options.IngressInvocationHandleOptions) error {
	path, err := makeAttachUrl(params)
	if err != nil {
		return err
	}
	return c.do(ctx, http.MethodGet, fmt.Sprintf("%s/attach", path), restate.Void{}, output, "", nil, 0, "", nil, attachOpts.Codec)
}

func (c *Client) Output(ctx context.Context, params IngressAttachParams, output any, outputOpts options.IngressInvocationHandleOptions) error {
	path, err := makeAttachUrl(params)
	if err != nil {
		return err
	}
	return c.do(ctx, http.MethodGet, fmt.Sprintf("%s/output", path), restate.Void{}, output, "", nil, 0, "", nil, outputOpts.Codec)
}

func (c *Client) do(ctx context.Context, httpMethod, path string, requestData any, responseData any, idempotencyKey string, headers map[string]string,
	delay time.Duration, limitKey string,
	inputCodec encoding.Codec, outputCodec encoding.Codec) error {
	// Set input/output codec
	if inputCodec == nil {
		inputCodec = c.clientOpts.Codec
	}
	if inputCodec == nil {
		inputCodec = encoding.JSONCodec
	}
	if outputCodec == nil {
		outputCodec = c.clientOpts.Codec
	}
	if outputCodec == nil {
		outputCodec = encoding.JSONCodec
	}

	// marshal the request data if provided
	requestBodyBuf, err := encoding.Marshal(inputCodec, requestData)
	if err != nil {
		return fmt.Errorf("failed to marshal request data: %w", err)
	}
	requestBody := bytes.NewBuffer(requestBodyBuf)

	// build the http request
	url := fmt.Sprintf("%s%s", c.baseUri, path)
	if delay != 0 {
		url = fmt.Sprintf("%s?%s=%dms", url, delayQuery, delay/time.Millisecond)
	}
	req, err := http.NewRequest(httpMethod, url, requestBody)
	if err != nil {
		return err
	}
	req = req.WithContext(ctx)

	// Figure out the content type
	inputPayloadMetadata := encoding.InputPayloadFor(inputCodec, requestData)
	if inputPayloadMetadata != nil && inputPayloadMetadata.ContentType != nil {
		req.Header.Set("Content-Type", *inputPayloadMetadata.ContentType)
	}

	// Add various headers
	if c.clientOpts.AuthKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.clientOpts.AuthKey)
	}
	if idempotencyKey != "" {
		req.Header.Set(idempotencyKeyHeader, idempotencyKey)
	}
	if limitKey != "" {
		req.Header.Set(limitKeyHeader, limitKey)
	}
	if headers != nil {
		for name, value := range headers {
			req.Header.Set(name, value)
		}
	}

	// make the call
	httpClient := http.DefaultClient
	if c.clientOpts.HttpClient != nil {
		httpClient = c.clientOpts.HttpClient
	}
	res, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("failed to read response resBody: %w", err)
	}

	// deal with error response
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		var bodyStr string
		if len(resBody) > 0 {
			bodyStr = string(resBody)
		} else {
			bodyStr = "<empty response resBody>"
		}
		var rerr restateError
		if len(resBody) > 0 {
			if err = json.Unmarshal(resBody, &rerr); err != nil {
				return fmt.Errorf("failed to unmarshal error response: %w: %s", err, bodyStr)
			}
		} else {
			rerr = restateError{
				Message: bodyStr,
				Code:    res.StatusCode,
			}
		}
		switch res.StatusCode {
		case http.StatusNotFound:
			return newInvocationNotFoundError(&rerr)
		case 470:
			return newInvocationNotReadyError(&rerr)
		case http.StatusInternalServerError:
			return newGenericError(&rerr)
		}
		return fmt.Errorf("request failed with unexpected status %s: %s", res.Status, bodyStr)
	}

	if responseData != nil {
		if err = encoding.Unmarshal(outputCodec, resBody, responseData); err != nil {
			return fmt.Errorf("failed to unmarshal response data: %w", err)
		}
	}

	return nil
}

func makeIngressUrl(params IngressParams, scope string, send bool) string {
	if scope != "" {
		verb := "call"
		if send {
			verb = "send"
		}
		switch {
		case params.Key != "":
			return fmt.Sprintf("/restate/scope/%s/%s/%s/%s/%s", scope, verb, params.Service, params.Key, params.Handler)
		default:
			return fmt.Sprintf("/restate/scope/%s/%s/%s/%s", scope, verb, params.Service, params.Handler)
		}
	}

	switch {
	case params.Key != "":
		if send {
			return fmt.Sprintf("/%s/%s/%s/send", params.Service, params.Key, params.Handler)
		}
		return fmt.Sprintf("/%s/%s/%s", params.Service, params.Key, params.Handler)
	default:
		if send {
			return fmt.Sprintf("/%s/%s/send", params.Service, params.Handler)
		}
		return fmt.Sprintf("/%s/%s", params.Service, params.Handler)
	}
}

func makeAttachUrl(params IngressAttachParams) (string, error) {
	switch {
	case params.InvocationID != "":
		return fmt.Sprintf("/restate/invocation/%s", params.InvocationID), nil
	case params.ObjectKey != "" && params.IdempotencyKey != "" && params.ServiceName != "" && params.MethodName != "":
		return fmt.Sprintf("/restate/invocation/%s/%s/%s/%s", params.ServiceName, params.ObjectKey, params.MethodName, params.IdempotencyKey), nil
	case params.WorkflowID != "" && params.ServiceName != "":
		return fmt.Sprintf("/restate/workflow/%s/%s", params.ServiceName, params.WorkflowID), nil
	case params.ServiceName != "" && params.MethodName != "" && params.IdempotencyKey != "":
		return fmt.Sprintf("/restate/invocation/%s/%s/%s", params.ServiceName, params.MethodName, params.IdempotencyKey), nil
	default:
		return "", errors.New("missing ingress attach params")
	}
}
