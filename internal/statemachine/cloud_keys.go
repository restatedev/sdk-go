package statemachine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/tetratelabs/wazero/api"
	"google.golang.org/protobuf/proto"

	pbinternal "github.com/restatedev/sdk-go/internal/generated"
)

// Restate Cloud keys. Parsing is done by the shared core (feature cloud_keys);
// the environment is read here, since the wasm module has no access to it.
const (
	ConnectKeyEnv = "RESTATE_CONNECT_KEY"
	IngressKeyEnv = "RESTATE_INGRESS_KEY"

	// ConnectKeyURL is where to get a connect key (Cloud UI, Development → Configure SDK).
	ConnectKeyURL = "https://cloud.restate.dev/to/developers/integration#configure-sdk"
	// IngressKeyURL is where to get an ingress key (Cloud UI, Development → Configure Client).
	IngressKeyURL = "https://cloud.restate.dev/to/developers/integration#configure-client"
)

// ConnectKeyFromEnv reads and parses RESTATE_CONNECT_KEY. It returns nil, nil when the variable is unset or empty.
func ConnectKeyFromEnv(ctx context.Context) (*pbinternal.ConnectKey, error) {
	value := strings.TrimSpace(os.Getenv(ConnectKeyEnv))
	if value == "" {
		return nil, nil
	}
	return parseConnectKey(ctx, value)
}

// IngressKeyFromEnv reads and parses RESTATE_INGRESS_KEY. It returns nil, nil when the variable is unset or empty.
func IngressKeyFromEnv(ctx context.Context) (*pbinternal.IngressKey, error) {
	value := strings.TrimSpace(os.Getenv(IngressKeyEnv))
	if value == "" {
		return nil, nil
	}
	return parseIngressKey(ctx, value)
}

// parseConnectKey parses a Restate Cloud connect key.
func parseConnectKey(ctx context.Context, key string) (*pbinternal.ConnectKey, error) {
	output := pbinternal.ParseConnectKeyReturn{}
	if err := callCloudKeyParser(ctx, "parse_connect_key", func(c *Core) api.Function { return c.parseConnectKey }, key, &output); err != nil {
		return nil, err
	}
	if output.HasFailure() {
		return nil, errors.New(output.GetFailure())
	}
	return output.GetOk(), nil
}

// parseIngressKey parses a Restate Cloud ingress key. A connect key carrying an API key is accepted too.
func parseIngressKey(ctx context.Context, key string) (*pbinternal.IngressKey, error) {
	output := pbinternal.ParseIngressKeyReturn{}
	if err := callCloudKeyParser(ctx, "parse_ingress_key", func(c *Core) api.Function { return c.parseIngressKey }, key, &output); err != nil {
		return nil, err
	}
	if output.HasFailure() {
		return nil, errors.New(output.GetFailure())
	}
	return output.GetOk(), nil
}

func callCloudKeyParser(ctx context.Context, name string, fn func(*Core) api.Function, key string, output proto.Message) error {
	core, err := NewCore(ctx)
	if err != nil {
		return err
	}
	defer core.Close(ctx)

	core.coreMutex.Lock()
	defer core.coreMutex.Unlock()

	params := pbinternal.CloudKeyParameters{}
	params.SetKey(key)
	inputPtr, inputLen := core.transferInputStructToWasmMemory(ctx, &params)

	core.callStack[0] = inputPtr
	core.callStack[1] = inputLen
	if err := fn(core).CallWithStack(ctx, core.callStack); err != nil {
		return fmt.Errorf("error when calling %s: %w", name, err)
	}
	core.transferOutputStructFromWasmMemory(ctx, core.callStack[0], output)
	return nil
}
