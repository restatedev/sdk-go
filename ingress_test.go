package restate_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/restatedev/sdk-go"
)

const (
	myService        = "MyService"
	myHandler        = "myHandler"
	myObjectKey      = "myObjectKey"
	myWorkflowId     = "myWorkflowId"
	idempotencyKey   = "itemKey"
	invocationId     = "inv_1"
	invocationStatus = "Accepted"
	run              = "run"
)

var (
	headers = map[string]string{
		"key1": "value1",
		"key2": "value2",
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

	_, err := restate.IngressService[map[string]any, any](myService, myHandler,
		restate.WithBaseUrl(m.URL)).
		Request(context.Background(), input,
			restate.WithIdempotencyKey(idempotencyKey),
			restate.WithHeaders(headers),
		)
	require.NoError(t, err)
	m.AssertMethod(t, http.MethodPost)
	m.AssertPath(t, fmt.Sprintf("/%s/%s", myService, myHandler))
	m.AssertBody(t, payload)
	m.AssertHeaders(t, headers)
	m.AssertQuery(t, nil)
}

func TestServiceSend(t *testing.T) {
	// curl localhost:8080/MyService/myHandler/send --json '{"name": "Mary", "age": 25}'
	m := newMockIngressServer()
	defer m.Close()

	var input map[string]any
	payload := []byte(`{"name":"Mary","age":25}`)
	require.NoError(t, json.Unmarshal(payload, &input))

	inv := restate.IngressServiceSend[map[string]any](myService, myHandler,
		restate.WithBaseUrl(m.URL)).
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

	_, err := restate.IngressObject[map[string]any, any](myService, myObjectKey, myHandler,
		restate.WithBaseUrl(m.URL)).
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

	inv := restate.IngressObjectSend[map[string]any](myService, myObjectKey, myHandler,
		restate.WithBaseUrl(m.URL)).
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

	_, err := restate.IngressWorkflow[map[string]any, any](myService, myWorkflowId, run,
		restate.WithBaseUrl(m.URL)).
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

	inv := restate.IngressWorkflowSend[map[string]any](myService, myWorkflowId, myHandler,
		restate.WithBaseUrl(m.URL)).
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

	_, err := restate.IngressAttachInvocation[any](invocationId,
		restate.WithBaseUrl(m.URL)).
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

	_, err := restate.IngressAttachInvocation[any](invocationId,
		restate.WithBaseUrl(m.URL)).
		Output(context.Background())
	require.NoError(t, err)
	m.AssertMethod(t, http.MethodGet)
	m.AssertPath(t, fmt.Sprintf("/restate/invocation/%s/output", invocationId))
	m.AssertBody(t, nil)
}

func TestAdminInvocationCancelByInvocationID(t *testing.T) {
	//curl -X DELETE localhost:9070/invocations/invocationId?mode=Cancel
	m := newMockIngressServer()
	defer m.Close()

	err := restate.IngressAttachInvocation[any](invocationId,
		restate.WithBaseUrl(m.URL)).
		Cancel(context.Background())
	require.NoError(t, err)
	m.AssertMethod(t, http.MethodDelete)
	m.AssertPath(t, fmt.Sprintf("/invocations/%s", invocationId))
	m.AssertBody(t, nil)
	m.AssertQuery(t, map[string]string{"mode": "Cancel"})
}

func TestAdminInvocationKillByInvocationID(t *testing.T) {
	//curl -X DELETE localhost:9070/invocations/invocationId?mode=Kill
	m := newMockIngressServer()
	defer m.Close()

	err := restate.IngressAttachInvocation[any](invocationId,
		restate.WithBaseUrl(m.URL)).
		Cancel(context.Background(), restate.WithCancelMode(restate.CancelModeKill))
	require.NoError(t, err)
	m.AssertMethod(t, http.MethodDelete)
	m.AssertPath(t, fmt.Sprintf("/invocations/%s", invocationId))
	m.AssertBody(t, nil)
	m.AssertQuery(t, map[string]string{"mode": "Kill"})
}

func TestAdminInvocationPurgeByInvocationID(t *testing.T) {
	//curl -X DELETE localhost:9070/invocations/invocationId?mode=Purge
	m := newMockIngressServer()
	defer m.Close()

	err := restate.IngressAttachInvocation[any](invocationId,
		restate.WithBaseUrl(m.URL)).
		Cancel(context.Background(), restate.WithCancelMode(restate.CancelModePurge))
	require.NoError(t, err)
	m.AssertMethod(t, http.MethodDelete)
	m.AssertPath(t, fmt.Sprintf("/invocations/%s", invocationId))
	m.AssertBody(t, nil)
	m.AssertQuery(t, map[string]string{"mode": "Purge"})
}

func TestServiceAttachByIdempotencyKey(t *testing.T) {
	//curl localhost:8080/restate/invocation/MyService/myHandler/myIdempotencyKey/attach
	m := newMockIngressServer()
	defer m.Close()

	_, err := restate.IngressAttachService[any](myService, myHandler, idempotencyKey,
		restate.WithBaseUrl(m.URL)).
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

	_, err := restate.IngressAttachService[any](myService, myHandler, idempotencyKey,
		restate.WithBaseUrl(m.URL)).
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

	_, err := restate.IngressAttachObject[any](myService, myObjectKey, myHandler, idempotencyKey,
		restate.WithBaseUrl(m.URL)).
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

	_, err := restate.IngressAttachObject[any](myService, myObjectKey, myHandler, idempotencyKey,
		restate.WithBaseUrl(m.URL)).
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

	_, err := restate.IngressAttachWorkflow[any](myService, myWorkflowId,
		restate.WithBaseUrl(m.URL)).
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

	_, err := restate.IngressAttachWorkflow[any](myService, myWorkflowId,
		restate.WithBaseUrl(m.URL)).
		Output(context.Background())
	require.NoError(t, err)
	m.AssertMethod(t, http.MethodGet)
	m.AssertPath(t, fmt.Sprintf("/restate/workflow/%s/%s/output", myService, myWorkflowId))
	m.AssertBody(t, nil)
}
