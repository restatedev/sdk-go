package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/restatedev/sdk-go/encoding"
	"github.com/restatedev/sdk-go/internal/options"
)

type ingressContextKey struct{}

func Connect(ctx context.Context, ingressURL string, opts ...options.ConnectOption) (context.Context, error) {
	o := options.ConnectOptions{}
	for _, opt := range opts {
		opt.BeforeConnect(&o)
	}
	if o.Client == nil {
		o.Client = http.DefaultClient
	}

	url, err := url.Parse(ingressURL)
	if err != nil {
		return nil, err
	}

	resp, err := o.Client.Get(url.JoinPath("restate", "health").String())
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Ingress is not healthy: status %d", resp.StatusCode)
	}

	return context.WithValue(ctx, ingressContextKey{}, &connection{o, url}), nil
}

type connection struct {
	options.ConnectOptions
	url *url.URL
}

func fromContext(ctx context.Context) (*connection, bool) {
	if val := ctx.Value(ingressContextKey{}); val != nil {
		c, ok := val.(*connection)
		return c, ok
	}
	return nil, false
}

func fromContextOrPanic(ctx context.Context) *connection {
	conn, ok := fromContext(ctx)
	if !ok {
		panic("Not connected to Restate ingress; provided ctx must have been returned from client.Connect")
	}

	return conn
}

// Client represents all the different ways you can invoke a particular service-method.
type IngressClient[I any, O any] interface {
	// RequestFuture makes a call and returns a handle on a future response
	RequestFuture(input I, options ...options.RequestOption) IngressResponseFuture[O]
	// Request makes a call and blocks on getting the response
	Request(input I, options ...options.RequestOption) (O, error)
	IngressSendClient[I]
}

type ingressClient[I any, O any] struct {
	ctx  context.Context
	conn *connection
	opts options.IngressClientOptions
	url  *url.URL
}

func (c *ingressClient[I, O]) RequestFuture(input I, opts ...options.RequestOption) IngressResponseFuture[O] {
	o := options.RequestOptions{}
	for _, opt := range opts {
		opt.BeforeRequest(&o)
	}

	headers := make(http.Header, len(c.conn.Headers)+len(o.Headers)+1)
	for k, v := range c.conn.Headers {
		headers.Add(k, v)
	}
	for k, v := range o.Headers {
		headers.Add(k, v)
	}
	if o.IdempotencyKey != "" {
		headers.Set("Idempotency-Key", o.IdempotencyKey)
	}

	done := make(chan struct{})
	f := &ingressResponseFuture[O]{done: done, codec: c.opts.Codec}
	go func() {
		defer close(done)

		data, err := encoding.Marshal(c.opts.Codec, input)
		if err != nil {
			f.r.err = err
			return
		}

		if len(data) > 0 {
			var i I
			if p := encoding.InputPayloadFor(c.opts.Codec, i); p != nil && p.ContentType != nil {
				headers.Add("Content-Type", *p.ContentType)
			}
		}

		request, err := http.NewRequestWithContext(c.ctx, "POST", c.url.String(), bytes.NewReader(data))
		if err != nil {
			f.r.err = err
			return
		}
		request.Header = headers

		f.r.Response, f.r.err = c.conn.Client.Do(request)
	}()

	return f
}

func (c *ingressClient[I, O]) Request(input I, opts ...options.RequestOption) (O, error) {
	return c.RequestFuture(input, opts...).Response()
}

func (c *ingressClient[I, O]) Send(input I, opts ...options.SendOption) (Send, error) {
	o := options.SendOptions{}
	for _, opt := range opts {
		opt.BeforeSend(&o)
	}

	headers := make(http.Header, len(c.conn.Headers)+len(o.Headers)+2)
	for k, v := range c.conn.Headers {
		headers.Add(k, v)
	}
	for k, v := range o.Headers {
		headers.Add(k, v)
	}
	if o.IdempotencyKey != "" {
		headers.Set("Idempotency-Key", o.IdempotencyKey)
	}

	data, err := encoding.Marshal(c.opts.Codec, input)
	if err != nil {
		return Send{}, err
	}

	if len(data) > 0 {
		var i I
		if p := encoding.InputPayloadFor(c.opts.Codec, i); p != nil && p.ContentType != nil {
			headers.Add("Content-Type", *p.ContentType)
		}
	}

	url := c.url.JoinPath("send")
	url.Query().Add("delay", fmt.Sprintf("%dms", o.Delay.Milliseconds()))

	request, err := http.NewRequestWithContext(c.ctx, "POST", url.String(), bytes.NewReader(data))
	if err != nil {
		return Send{}, err
	}
	request.Header = headers

	resp, err := c.conn.Client.Do(request)
	if err != nil {
		return Send{}, err
	}

	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(resp.Body)
		return Send{}, fmt.Errorf("Send request failed: status %d\n%s", resp.StatusCode, string(body))
	}

	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return Send{}, err
	}

	send := Send{Attachable: o.IdempotencyKey != ""}
	return send, encoding.Unmarshal(encoding.JSONCodec, bytes, &send)
}

// IngressResponseFuture is a handle on a potentially not-yet completed outbound call.
type IngressResponseFuture[O any] interface {
	// Response blocks on the response to the call and returns it
	Response() (O, error)
}

type ingressResponseFuture[O any] struct {
	done  <-chan struct{} // guards access to r
	r     response
	codec encoding.Codec
}

func (f *ingressResponseFuture[O]) Response() (o O, err error) {
	<-f.done

	return o, f.r.Decode(f.codec, &o)
}

type response struct {
	*http.Response
	err error
}

func (r *response) Decode(codec encoding.Codec, v any) error {
	if r.err != nil {
		return r.err
	}

	defer r.Body.Close()

	if r.StatusCode < 200 || r.StatusCode > 299 {
		body, _ := io.ReadAll(r.Body)
		return fmt.Errorf("Request failed: status %d\n%s", r.StatusCode, string(body))
	}

	bytes, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}

	return encoding.Unmarshal(codec, bytes, v)
}

// IngressSendClient allows making one-way invocations
type IngressSendClient[I any] interface {
	// Send makes a one-way call which is executed in the background
	Send(input I, options ...options.SendOption) (Send, error)
}

type SendStatus uint16

const (
	SendStatusUnknown SendStatus = iota
	SendStatusAccepted
	SendStatusPreviouslyAccepted
)

func (s *SendStatus) UnmarshalJSON(data []byte) (err error) {
	var ss string
	if err := json.Unmarshal(data, &ss); err != nil {
		return err
	}
	switch ss {
	case "Accepted":
		*s = SendStatusAccepted
	case "PreviouslyAccepted":
		*s = SendStatusPreviouslyAccepted
	default:
		*s = SendStatusUnknown
	}
	return nil
}

// Send is an object describing a submitted invocation to Restate, which can be attached to with [Attach]
type Send struct {
	InvocationId string     `json:"invocationID"`
	Status       SendStatus `json:"status"`
	Attachable   bool       `json:"-"`
}

func (s Send) attachable() bool {
	return s.Attachable
}

func (s Send) attachUrl(connURL *url.URL) *url.URL {
	return connURL.JoinPath("restate", "invocation", s.InvocationId, "attach")
}

func (s Send) outputUrl(connURL *url.URL) *url.URL {
	return connURL.JoinPath("restate", "invocation", s.InvocationId, "output")
}

var _ Attacher = Send{}

// Attacher is implemented by [Send], [WorkflowSubmission] and [WorkflowIdentifier]
type Attacher interface {
	attachable() bool
	attachUrl(connURL *url.URL) *url.URL
	outputUrl(connURL *url.URL) *url.URL
}

// Attach attaches to the attachable invocation and returns its response. The invocation must have been created with an idempotency key
// or by a workflow submission.
// It must be called with a context returned from [Connect]
func Attach[O any](ctx context.Context, attacher Attacher, opts ...options.IngressClientOption) (o O, err error) {
	conn := fromContextOrPanic(ctx)

	if !attacher.attachable() {
		return o, fmt.Errorf("Unable to fetch the result.\nA service's result is stored only when an idempotencyKey is supplied when invoking the service, or if its a workflow submission.")
	}

	os := options.IngressClientOptions{}
	for _, opt := range opts {
		opt.BeforeIngressClient(&os)
	}
	if os.Codec == nil {
		os.Codec = encoding.JSONCodec
	}

	headers := make(http.Header, len(conn.Headers)+1)
	for k, v := range conn.Headers {
		headers.Add(k, v)
	}

	url := attacher.attachUrl(conn.url)

	request, err := http.NewRequestWithContext(ctx, "GET", url.String(), nil)
	if err != nil {
		return o, err
	}
	request.Header = headers

	resp, err := conn.Client.Do(request)
	if err != nil {
		return o, err
	}

	return o, (&response{Response: resp}).Decode(os.Codec, &o)
}

// GetOutput gets the output of the attachable invocation and returns its response if it has completed. The invocation must have been created with an idempotency key
// or by a workflow submission.
// It must be called with a context returned from [Connect].
func GetOutput[O any](ctx context.Context, attacher Attacher, opts ...options.IngressClientOption) (o O, ready bool, err error) {
	conn := fromContextOrPanic(ctx)

	if !attacher.attachable() {
		return o, false, fmt.Errorf("Unable to fetch the result.\nA service's result is stored only when an idempotencyKey is supplied when invoking the service, or if its a workflow submission.")
	}

	os := options.IngressClientOptions{}
	for _, opt := range opts {
		opt.BeforeIngressClient(&os)
	}
	if os.Codec == nil {
		os.Codec = encoding.JSONCodec
	}

	headers := make(http.Header, len(conn.Headers)+1)
	for k, v := range conn.Headers {
		headers.Add(k, v)
	}

	url := attacher.outputUrl(conn.url)

	request, err := http.NewRequestWithContext(ctx, "GET", url.String(), nil)
	if err != nil {
		return o, false, err
	}
	request.Header = headers

	resp, err := conn.Client.Do(request)
	if err != nil {
		return o, false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 470 {
		// special status code used by restate to say that the result is not ready
		return o, false, nil
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(resp.Body)
		return o, false, fmt.Errorf("Request failed: status %d\n%s", resp.StatusCode, string(body))
	}

	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return o, false, err
	}

	if err := encoding.Unmarshal(os.Codec, bytes, &o); err != nil {
		return o, false, err
	}

	return o, true, nil
}

func getClient[O any](ctx context.Context, conn *connection, url *url.URL, opts ...options.IngressClientOption) IngressClient[any, O] {
	o := options.IngressClientOptions{}
	for _, opt := range opts {
		opt.BeforeIngressClient(&o)
	}
	if o.Codec == nil {
		o.Codec = encoding.JSONCodec
	}

	return &ingressClient[any, O]{ctx, conn, o, url}
}

// Service gets a Service request client by service and method name
// It must be called with a context returned from [Connect]
func Service[O any](ctx context.Context, service string, method string, opts ...options.IngressClientOption) IngressClient[any, O] {
	conn := fromContextOrPanic(ctx)
	url := conn.url.JoinPath(service, method)
	return getClient[O](ctx, conn, url, opts...)
}

// Service gets a Service send client by service and method name
// It must be called with a context returned from [Connect]
func ServiceSend(ctx context.Context, service string, method string, opts ...options.IngressClientOption) IngressSendClient[any] {
	return Service[any](ctx, service, method, opts...)
}

// Object gets an Object request client by service name, key and method name
// It must be called with a context returned from [Connect]
func Object[O any](ctx context.Context, service string, key string, method string, opts ...options.IngressClientOption) IngressClient[any, O] {
	conn := fromContextOrPanic(ctx)
	url := conn.url.JoinPath(service, key, method)

	return getClient[O](ctx, conn, url, opts...)
}

// ObjectSend gets an Object send client by service name, key and method name
// It must be called with a context returned from [Connect]
func ObjectSend(ctx context.Context, service string, key string, method string, opts ...options.IngressClientOption) IngressSendClient[any] {
	return Object[any](ctx, service, key, method, opts...)
}

// ResolveAwakeable allows an awakeable to be resolved with a particular value.
// It must be called with a context returned from [Connect]
func ResolveAwakeable[T any](ctx context.Context, id string, value T, opts ...options.ResolveAwakeableOption) error {
	conn := fromContextOrPanic(ctx)

	o := options.ResolveAwakeableOptions{}
	for _, opt := range opts {
		opt.BeforeResolveAwakeable(&o)
	}
	if o.Codec == nil {
		o.Codec = encoding.JSONCodec
	}

	headers := make(http.Header, len(conn.Headers)+1)
	for k, v := range conn.Headers {
		headers.Add(k, v)
	}

	data, err := encoding.Marshal(o.Codec, value)
	if err != nil {
		return err
	}

	url := conn.url.JoinPath("restate", "a", id, "resolve")

	request, err := http.NewRequestWithContext(ctx, "POST", url.String(), bytes.NewReader(data))
	if err != nil {
		return err
	}
	request.Header = headers

	resp, err := conn.Client.Do(request)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Resolve awakeable request failed: status %d\n%s", resp.StatusCode, string(body))
	}
	return nil
}

// RejectAwakeable allows an awakeable to be rejected with a particular error.
// It must be called with a context returned from [Connect]
func RejectAwakeable(ctx context.Context, id string, reason error) error {
	conn := fromContextOrPanic(ctx)

	headers := make(http.Header, len(conn.Headers)+1)
	for k, v := range conn.Headers {
		headers.Add(k, v)
	}

	data := []byte(reason.Error())

	url := conn.url.JoinPath("restate", "a", id, "reject")

	request, err := http.NewRequestWithContext(ctx, "POST", url.String(), bytes.NewReader(data))
	if err != nil {
		return err
	}
	request.Header = headers

	resp, err := conn.Client.Do(request)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Reject awakeable request failed: status %d\n%s", resp.StatusCode, string(body))
	}
	return nil
}
