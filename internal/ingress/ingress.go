package ingress

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	restate "github.com/restatedev/sdk-go"
	"github.com/restatedev/sdk-go/encoding"
	"io"
	"net/http"
	"time"

	"github.com/restatedev/sdk-go/internal/options"
)

const (
	idempotencyKeyHeader = "idempotency-key"
	delayQuery           = "delay"
)

// Client is an ingress client used to initiate Restate invocations outside a Restate context.
type Client struct {
	baseUri    string
	clientOpts options.IngressClientOptions
}

type IngressParams struct {
	ServiceName string
	HandlerName string
	ObjectKey   string
	WorkflowID  string
}

type IngressAttachParams struct {
	ServiceName    string
	MethodName     string
	ObjectKey      string
	InvocationID   string
	IdempotencyKey string
	WorkflowID     string
}

type ingressOpts struct {
	IdempotencyKey string
	Headers        map[string]string
	Delay          time.Duration
	Codec          encoding.PayloadCodec
}

func NewClient(baseUri string, opts options.IngressClientOptions) *Client {
	return &Client{
		baseUri:    baseUri,
		clientOpts: opts,
	}
}

func (c *Client) Request(ctx context.Context, params IngressParams, input, output any, reqOpts options.IngressRequestOptions) error {
	return c.do(ctx, http.MethodPost, makeIngressUrl(params), input, output, requestOptionsToIngressOpts(reqOpts))
}

func (c *Client) Send(ctx context.Context, params IngressParams, input any, sendOpts options.IngressSendOptions) Invocation {
	url := fmt.Sprintf("%s/%s", makeIngressUrl(params), "send")
	var output Invocation
	err := c.do(ctx, http.MethodPost, url, input, &output, sendOptionsToIngressOpts(sendOpts))
	if err != nil {
		output.Error = err
	}
	return output
}

func (c *Client) Attach(ctx context.Context, params IngressAttachParams, output any) error {
	path, err := makeAttachUrl(params)
	if err != nil {
		return err
	}
	return c.do(ctx, http.MethodGet, fmt.Sprintf("%s/attach", path), restate.Void{}, output, ingressOpts{})
}

func (c *Client) Output(ctx context.Context, params IngressAttachParams, output any) error {
	path, err := makeAttachUrl(params)
	if err != nil {
		return err
	}
	return c.do(ctx, http.MethodGet, fmt.Sprintf("%s/output", path), restate.Void{}, output, ingressOpts{})
}

func (c *Client) do(ctx context.Context, httpMethod, path string, requestData any, responseData any, opts ingressOpts) error {
	// Establish the codec to use
	codec := opts.Codec
	if codec == nil {
		codec = c.clientOpts.Codec
	}
	if codec == nil {
		codec = encoding.JSONCodec
	}

	// marshal the request data if provided
	requestBodyBuf, err := encoding.Marshal(codec, requestData)
	if err != nil {
		return fmt.Errorf("failed to marshal request data: %w", err)
	}
	requestBody := bytes.NewBuffer(requestBodyBuf)

	// build the http request
	url := fmt.Sprintf("%s%s", c.baseUri, path)
	if opts.Delay != 0 {
		url = fmt.Sprintf("%s?%s=%dms", url, delayQuery, opts.Delay/time.Millisecond)
	}
	req, err := http.NewRequest(httpMethod, url, requestBody)
	if err != nil {
		return err
	}
	req = req.WithContext(ctx)

	// Figure out the content type
	inputPayloadMetadata := encoding.InputPayloadFor(codec, requestData)
	if inputPayloadMetadata != nil && inputPayloadMetadata.ContentType != nil {
		req.Header.Set("Content-Type", *inputPayloadMetadata.ContentType)
	}

	// Add various headers
	if c.clientOpts.AuthKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.clientOpts.AuthKey)
	}
	if opts.IdempotencyKey != "" {
		req.Header.Set(idempotencyKeyHeader, opts.IdempotencyKey)
	}
	if opts.Headers != nil {
		for name, value := range opts.Headers {
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
		if err = encoding.Unmarshal(codec, resBody, responseData); err != nil {
			return fmt.Errorf("failed to unmarshal response data: %w", err)
		}
	}

	return nil
}

func requestOptionsToIngressOpts(reqOpts options.IngressRequestOptions) ingressOpts {
	return ingressOpts{
		IdempotencyKey: reqOpts.IdempotencyKey,
		Headers:        reqOpts.Headers,
		Codec:          reqOpts.Codec,
	}
}

func sendOptionsToIngressOpts(sendOpts options.IngressSendOptions) ingressOpts {
	return ingressOpts{
		IdempotencyKey: sendOpts.IdempotencyKey,
		Headers:        sendOpts.Headers,
		Delay:          sendOpts.Delay,
		Codec:          sendOpts.Codec,
	}
}

func makeIngressUrl(params IngressParams) string {
	switch {
	case params.ObjectKey != "":
		return fmt.Sprintf("/%s/%s/%s", params.ServiceName, params.ObjectKey, params.HandlerName)
	case params.WorkflowID != "":
		return fmt.Sprintf("/%s/%s/%s", params.ServiceName, params.WorkflowID, params.HandlerName)
	default:
		return fmt.Sprintf("/%s/%s", params.ServiceName, params.HandlerName)
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
