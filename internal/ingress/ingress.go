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

	"github.com/restatedev/sdk-go/internal/options"
)

const (
	idempotencyKeyHeader = "idempotency-key"
	delayQuery           = "delay"
)

// Client is an ingress client used to initiate Restate invocations outside of a Restate context.
type Client struct {
	baseUri    string
	ClientOpts options.IngressClientOptions
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
}

func NewClient(baseUri string, opts options.IngressClientOptions) *Client {
	return &Client{
		baseUri:    baseUri,
		ClientOpts: opts,
	}
}

func (c *Client) Request(ctx context.Context, params IngressParams, input, output any, reqOpts options.RequestOptions) error {
	return c.do(ctx, http.MethodPost, makeIngressUrl(params), input, output, requestOptionsToIngressOpts(reqOpts))
}

func (c *Client) Send(ctx context.Context, params IngressParams, input any, sendOpts options.SendOptions) Invocation {
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
	return c.do(ctx, http.MethodGet, fmt.Sprintf("%s/attach", path), nil, output, ingressOpts{})
}

func (c *Client) Output(ctx context.Context, params IngressAttachParams, output any) error {
	path, err := makeAttachUrl(params)
	if err != nil {
		return err
	}
	return c.do(ctx, http.MethodGet, fmt.Sprintf("%s/output", path), nil, output, ingressOpts{})
}

func (c *Client) do(ctx context.Context, httpMethod, path string, requestData any, responseData any, opts ingressOpts) error {
	// marshal the request data if provided
	var requestBody io.Reader
	if requestData != nil {
		byts, err := json.Marshal(&requestData)
		if err != nil {
			return fmt.Errorf("failed to marshal request data: %w", err)
		}
		requestBody = bytes.NewBuffer(byts)
	}

	// build the http request
	url := fmt.Sprintf("%s/%s", c.baseUri, path)
	if opts.Delay != 0 {
		url = fmt.Sprintf("%s?%s=%dms", url, delayQuery, opts.Delay/time.Millisecond)
	}
	req, err := http.NewRequest(httpMethod, url, requestBody)
	if err != nil {
		return err
	}
	req = req.WithContext(ctx)

	if c.ClientOpts.AuthKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.ClientOpts.AuthKey)
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
	if c.ClientOpts.HttpClient != nil {
		httpClient = c.ClientOpts.HttpClient
	}
	res, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// deal with error response
	var bodyStr string
	if len(body) > 0 {
		bodyStr = string(body)
	} else {
		bodyStr = "<empty response body>"
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		var rerr restateError
		if len(body) > 0 {
			if err = json.Unmarshal(body, &rerr); err != nil {
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
		if err = json.Unmarshal(body, &responseData); err != nil {
			return fmt.Errorf("failed to unmarshal response data: %w: %s", err, bodyStr)
		}
	}

	return nil
}

func requestOptionsToIngressOpts(reqOpts options.RequestOptions) ingressOpts {
	return ingressOpts{
		IdempotencyKey: reqOpts.IdempotencyKey,
		Headers:        reqOpts.Headers,
	}
}

func sendOptionsToIngressOpts(sendOpts options.SendOptions) ingressOpts {
	return ingressOpts{
		IdempotencyKey: sendOpts.IdempotencyKey,
		Headers:        sendOpts.Headers,
		Delay:          sendOpts.Delay,
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
