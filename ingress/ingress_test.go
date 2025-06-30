package ingress_test

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/restatedev/sdk-go/encoding"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/restatedev/sdk-go"
	"github.com/restatedev/sdk-go/ingress"
)

const (
	myService        = "MyService"
	myHandler        = "myHandler"
	myObjectKey      = "myObjectKey"
	myWorkflowId     = "myWorkflowId"
	authKey          = "authKey"
	idempotencyKey   = "itemKey"
	invocationId     = "inv_1"
	invocationStatus = "Accepted"
	run              = "run"
)

var (
	headers = map[string]string{
		"Authorization": "Bearer " + authKey,
		"key1":          "value1",
		"key2":          "value2",
	}
	query = map[string]string{
		"delay": "1ms",
	}
)

func TestServiceRequest(t *testing.T) {
	// curl localhost:8080/MyService/myHandler --json '{"name": "Mary", "age": 25}'
	m := newMockIngressServer()
	defer m.Close()

	var input map[string]any
	payload := []byte(`{"name":"Mary","age":25}`)
	require.NoError(t, json.Unmarshal(payload, &input))

	c := newIngressClient(m.URL)
	_, err := ingress.Service[map[string]any, any](c, myService, myHandler).
		Request(context.Background(), input,
			restate.WithIdempotencyKey(idempotencyKey),
			restate.WithHeaders(headers),
		)
	require.NoError(t, err)
	m.AssertMethod(t, http.MethodPost)
	m.AssertPath(t, fmt.Sprintf("/%s/%s", myService, myHandler))
	m.AssertBody(t, payload)
	m.AssertHeaders(t, headers)
	m.AssertContentType(t, "application/json")
	m.AssertQuery(t, nil)
}

func TestNoInput(t *testing.T) {
	// curl localhost:8080/MyService/myHandler --json '{"name": "Mary", "age": 25}'
	m := newMockIngressServer()
	defer m.Close()

	c := newIngressClient(m.URL)
	_, err := ingress.Service[encoding.Void, any](c, myService, myHandler).
		Request(context.Background(), encoding.Void{},
			restate.WithIdempotencyKey(idempotencyKey),
			restate.WithHeaders(headers),
		)
	require.NoError(t, err)
	m.AssertMethod(t, http.MethodPost)
	m.AssertPath(t, fmt.Sprintf("/%s/%s", myService, myHandler))
	m.AssertHeaders(t, headers)
	m.AssertNoBody(t)
	m.AssertNoContentType(t)
	m.AssertQuery(t, nil)
}

func TestServiceSend(t *testing.T) {
	// curl localhost:8080/MyService/myHandler/send --json '{"name": "Mary", "age": 25}'
	m := newMockIngressServer()
	defer m.Close()

	var input map[string]any
	payload := []byte(`{"name":"Mary","age":25}`)
	require.NoError(t, json.Unmarshal(payload, &input))

	c := newIngressClient(m.URL)
	inv := ingress.ServiceSend[map[string]any](c, myService, myHandler).
		Send(context.Background(), input,
			restate.WithIdempotencyKey(idempotencyKey),
			restate.WithHeaders(headers),
			restate.WithDelay(time.Millisecond),
		)
	require.NoError(t, inv.Error)
	require.Equal(t, invocationId, inv.Id)
	require.Equal(t, invocationStatus, inv.Status)
	m.AssertMethod(t, http.MethodPost)
	m.AssertPath(t, fmt.Sprintf("/%s/%s/send", myService, myHandler))
	m.AssertBody(t, payload)
	m.AssertHeaders(t, headers)
	m.AssertQuery(t, query)
}

func TestObjectRequest(t *testing.T) {
	// curl localhost:8080/MyVirtualObject/myObjectKey/myHandler --json '{"name": "Mary", "age": 25}'
	m := newMockIngressServer()
	defer m.Close()

	var input map[string]any
	payload := []byte(`{"name":"Mary","age":25}`)
	require.NoError(t, json.Unmarshal(payload, &input))

	c := newIngressClient(m.URL)
	_, err := ingress.Object[map[string]any, any](c, myService, myObjectKey, myHandler).
		Request(context.Background(), input,
			restate.WithIdempotencyKey(idempotencyKey),
			restate.WithHeaders(headers),
		)
	require.NoError(t, err)
	m.AssertMethod(t, http.MethodPost)
	m.AssertPath(t, fmt.Sprintf("/%s/%s/%s", myService, myObjectKey, myHandler))
	m.AssertBody(t, payload)
	m.AssertHeaders(t, headers)
	m.AssertQuery(t, nil)
}

func TestObjectSend(t *testing.T) {
	// curl localhost:8080/MyService/myObjectKey/myHandler/send --json '{"name": "Mary", "age": 25}'
	m := newMockIngressServer()
	defer m.Close()

	var input map[string]any
	payload := []byte(`{"name":"Mary","age":25}`)
	require.NoError(t, json.Unmarshal(payload, &input))

	c := newIngressClient(m.URL)
	inv := ingress.ObjectSend[map[string]any](c, myService, myObjectKey, myHandler).
		Send(context.Background(), input,
			restate.WithIdempotencyKey(idempotencyKey),
			restate.WithHeaders(headers),
			restate.WithDelay(time.Millisecond),
		)
	require.NoError(t, inv.Error)
	require.Equal(t, invocationId, inv.Id)
	require.Equal(t, invocationStatus, inv.Status)
	m.AssertMethod(t, http.MethodPost)
	m.AssertPath(t, fmt.Sprintf("/%s/%s/%s/send", myService, myObjectKey, myHandler))
	m.AssertBody(t, payload)
	m.AssertHeaders(t, headers)
	m.AssertQuery(t, query)
}

func TestWorkflowRun(t *testing.T) {
	// curl localhost:8080/MyWorkflow/myWorkflowId/run --json '{"name": "Mary", "age": 25}'
	m := newMockIngressServer()
	defer m.Close()

	var input map[string]any
	payload := []byte(`{"name":"Mary","age":25}`)
	require.NoError(t, json.Unmarshal(payload, &input))

	c := newIngressClient(m.URL)
	_, err := ingress.Workflow[map[string]any, any](c, myService, myWorkflowId, run).
		Request(context.Background(), input,
			restate.WithIdempotencyKey(idempotencyKey),
			restate.WithHeaders(headers),
		)
	require.NoError(t, err)
	m.AssertMethod(t, http.MethodPost)
	m.AssertPath(t, fmt.Sprintf("/%s/%s/%s", myService, myWorkflowId, run))
	m.AssertBody(t, payload)
	m.AssertHeaders(t, headers)
	m.AssertQuery(t, nil)
}

func TestWorkflowSend(t *testing.T) {
	// curl localhost:8080/MyService/myWorkflowId/myHandler/send --json '{"name": "Mary", "age": 25}'
	m := newMockIngressServer()
	defer m.Close()

	var input map[string]any
	payload := []byte(`{"name":"Mary","age":25}`)
	require.NoError(t, json.Unmarshal(payload, &input))

	c := newIngressClient(m.URL)
	inv := ingress.WorkflowSend[map[string]any](c, myService, myWorkflowId, myHandler).
		Send(context.Background(), input,
			restate.WithIdempotencyKey(idempotencyKey),
			restate.WithHeaders(headers),
			restate.WithDelay(time.Millisecond),
		)
	require.NoError(t, inv.Error)
	require.Equal(t, invocationId, inv.Id)
	require.Equal(t, invocationStatus, inv.Status)
	m.AssertMethod(t, http.MethodPost)
	m.AssertPath(t, fmt.Sprintf("/%s/%s/%s/send", myService, myWorkflowId, myHandler))
	m.AssertBody(t, payload)
	m.AssertHeaders(t, headers)
	m.AssertQuery(t, query)
}

func TestInvocationAttachByInvocationID(t *testing.T) {
	//curl localhost:8080/restate/invocation/myInvocationId/attach
	m := newMockIngressServer()
	defer m.Close()

	c := newIngressClient(m.URL)
	_, err := ingress.AttachInvocation[any](c, invocationId).
		Attach(context.Background())
	require.NoError(t, err)
	m.AssertMethod(t, http.MethodGet)
	m.AssertPath(t, fmt.Sprintf("/restate/invocation/%s/attach", invocationId))
	m.AssertBody(t, nil)
}

func TestInvocationOutputByInvocationID(t *testing.T) {
	//curl localhost:8080/restate/invocation/myInvocationId/output
	m := newMockIngressServer()
	defer m.Close()

	c := newIngressClient(m.URL)
	_, err := ingress.AttachInvocation[any](c, invocationId).
		Output(context.Background())
	require.NoError(t, err)
	m.AssertMethod(t, http.MethodGet)
	m.AssertPath(t, fmt.Sprintf("/restate/invocation/%s/output", invocationId))
	m.AssertBody(t, nil)
}

func TestServiceAttachByIdempotencyKey(t *testing.T) {
	//curl localhost:8080/restate/invocation/MyService/myHandler/myIdempotencyKey/attach
	m := newMockIngressServer()
	defer m.Close()

	c := newIngressClient(m.URL)
	_, err := ingress.AttachService[any](c, myService, myHandler, idempotencyKey).
		Attach(context.Background())
	require.NoError(t, err)
	m.AssertMethod(t, http.MethodGet)
	m.AssertPath(t, fmt.Sprintf("/restate/invocation/%s/%s/%s/attach", myService, myHandler, idempotencyKey))
	m.AssertBody(t, nil)
}

func TestServiceOutputByIdempotencyKey(t *testing.T) {
	//curl localhost:8080/restate/invocation/MyService/myHandler/myIdempotencyKey/output
	m := newMockIngressServer()
	defer m.Close()

	c := newIngressClient(m.URL)
	_, err := ingress.AttachService[any](c, myService, myHandler, idempotencyKey).
		Output(context.Background())
	require.NoError(t, err)
	m.AssertMethod(t, http.MethodGet)
	m.AssertPath(t, fmt.Sprintf("/restate/invocation/%s/%s/%s/output", myService, myHandler, idempotencyKey))
	m.AssertBody(t, nil)
}

func TestObjectAttachByIdempotencyKey(t *testing.T) {
	//curl localhost:8080/restate/invocation/myObject/myKey/myHandler/myIdempotencyKey/attach
	m := newMockIngressServer()
	defer m.Close()

	c := newIngressClient(m.URL)
	_, err := ingress.AttachObject[any](c, myService, myObjectKey, myHandler, idempotencyKey).
		Attach(context.Background())
	require.NoError(t, err)
	m.AssertMethod(t, http.MethodGet)
	m.AssertPath(t, fmt.Sprintf("/restate/invocation/%s/%s/%s/%s/attach", myService, myObjectKey, myHandler, idempotencyKey))
	m.AssertBody(t, nil)
}

func TestObjectOutputByIdempotencyKey(t *testing.T) {
	//curl localhost:8080/restate/invocation/myObject/myKey/myHandler/myIdempotencyKey/output
	m := newMockIngressServer()
	defer m.Close()

	c := newIngressClient(m.URL)
	_, err := ingress.AttachObject[any](c, myService, myObjectKey, myHandler, idempotencyKey).
		Output(context.Background())
	require.NoError(t, err)
	m.AssertMethod(t, http.MethodGet)
	m.AssertPath(t, fmt.Sprintf("/restate/invocation/%s/%s/%s/%s/output", myService, myObjectKey, myHandler, idempotencyKey))
	m.AssertBody(t, nil)
}

func TestWorkflowAttach(t *testing.T) {
	//curl localhost:8080/restate/workflow/MyWorkflow/myWorkflowId/attach
	m := newMockIngressServer()
	defer m.Close()

	c := newIngressClient(m.URL)
	_, err := ingress.AttachWorkflow[any](c, myService, myWorkflowId).
		Attach(context.Background())
	require.NoError(t, err)
	m.AssertMethod(t, http.MethodGet)
	m.AssertPath(t, fmt.Sprintf("/restate/workflow/%s/%s/attach", myService, myWorkflowId))
	m.AssertBody(t, nil)
}

func TestWorkflowOutput(t *testing.T) {
	//curl localhost:8080/restate/workflow/MyWorkflow/myWorkflowId/output
	m := newMockIngressServer()
	defer m.Close()

	c := newIngressClient(m.URL)
	_, err := ingress.AttachWorkflow[any](c, myService, myWorkflowId).
		Output(context.Background())
	require.NoError(t, err)
	m.AssertMethod(t, http.MethodGet)
	m.AssertPath(t, fmt.Sprintf("/restate/workflow/%s/%s/output", myService, myWorkflowId))
	m.AssertBody(t, nil)
}

func newIngressClient(baseUri string) *ingress.Client {
	return ingress.NewClient(baseUri,
		restate.WithHttpClient(http.DefaultClient),
		restate.WithAuthKey(authKey))
}
