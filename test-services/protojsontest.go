package main

// This file contains E2E tests for PayloadOptions.unstable_serialization feature.
//
// # Background
//
// Some codecs (like protojson) may produce non-deterministic output - the same value
// can serialize to different bytes on different calls. During journal replay, Restate
// compares serialized bytes to verify determinism. If bytes differ, replay fails with
// a journal mismatch error.
//
// PayloadOptions.unstable_serialization=true tells Restate to skip this byte comparison,
// allowing non-deterministic codecs to work correctly.
//
// # Test Services
//
// 1. UnstableCodecTest - Uses a codec that implements IsUnstable()=true
//    Expected: Replay succeeds despite different serialized bytes
//
// 2. UnstableCodecFailTest - Uses the SAME non-deterministic codec but WITHOUT IsUnstable()
//    Expected: Replay FAILS with journal mismatch error
//
// # How to Run the Tests
//
// Prerequisites:
//   - Restate server running (e.g., in Docker/OrbStack)
//   - test-services running and registered with Restate
//
// ## Test 1: Verify PayloadOptions works (should SUCCEED)
//
//   # Start invocation (will suspend on awakeable)
//   curl -X POST "http://localhost:8080/UnstableCodecTest/test-key-1/setAndAwait/send" \
//     -H "Content-Type: application/json" -d '"test-value"'
//
//   # Get awakeable ID
//   curl -X POST "http://localhost:8080/UnstableCodecTest/test-key-1/getAwakeableId"
//
//   # Kill test-services to force replay on restart
//   pkill -f test-services
//
//   # Restart test-services
//   go run ./test-services
//
//   # Complete awakeable (triggers replay)
//   curl -X POST "http://localhost:8080/restate/awakeables/<AWAKEABLE_ID>/resolve" \
//     -H "Content-Type: application/json" -d '"completed"'
//
//   # Result: Invocation should complete successfully despite Set serializing
//   # different bytes on replay (because unstable_serialization=true)
//
// ## Test 2: Verify failure without PayloadOptions (should FAIL)
//
//   # Same steps as above but use UnstableCodecFailTest instead
//   curl -X POST "http://localhost:8080/UnstableCodecFailTest/test-key-1/setAndAwait/send" \
//     -H "Content-Type: application/json" -d '"test-value"'
//
//   # After restart and awakeable completion, invocation should fail with
//   # journal mismatch error because unstable_serialization=false

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"

	restate "github.com/restatedev/sdk-go"
	"github.com/restatedev/sdk-go/encoding"
)

// baseUnstableCodec is a codec that intentionally produces different bytes on each Marshal call.
// It wraps JSON but adds a random nonce to force non-deterministic output.
type baseUnstableCodec struct{}

func (u baseUnstableCodec) Marshal(v any) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	// Add random bytes to make output non-deterministic
	randomBytes := make([]byte, 4)
	rand.Read(randomBytes)
	// Wrap in an object with the data and a random nonce
	wrapper := map[string]any{
		"data":  json.RawMessage(data),
		"nonce": hex.EncodeToString(randomBytes),
	}
	return json.Marshal(wrapper)
}

func (u baseUnstableCodec) Unmarshal(data []byte, v any) error {
	var wrapper map[string]json.RawMessage
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return err
	}
	return json.Unmarshal(wrapper["data"], v)
}

func (u baseUnstableCodec) InputPayload(i any) *encoding.InputPayload {
	return encoding.JSONCodec.InputPayload(i)
}

func (u baseUnstableCodec) OutputPayload(o any) *encoding.OutputPayload {
	return encoding.JSONCodec.OutputPayload(o)
}

// unstableCodecWithFlag is the non-deterministic codec that correctly implements
// UnstableSerializer interface to declare itself as unstable.
type unstableCodecWithFlag struct {
	baseUnstableCodec
}

// IsUnstable implements encoding.UnstableSerializer interface.
// Returns true to tell Restate this codec produces non-deterministic output.
func (u unstableCodecWithFlag) IsUnstable() bool {
	return true
}

// unstableCodecWithoutFlag is the SAME non-deterministic codec but does NOT
// implement UnstableSerializer. This will cause journal mismatch on replay.
type unstableCodecWithoutFlag struct {
	baseUnstableCodec
}

// Note: unstableCodecWithoutFlag intentionally does NOT implement IsUnstable()
// This means IsUnstableSerialization() returns false, and PayloadOptions will
// have unstable_serialization=false, causing replay to fail.

var (
	// UnstableCodec - use this for successful replay (has IsUnstable=true)
	UnstableCodec encoding.PayloadCodec = unstableCodecWithFlag{}

	// UnstableCodecNoFlag - use this to demonstrate failure (no IsUnstable)
	UnstableCodecNoFlag encoding.PayloadCodec = unstableCodecWithoutFlag{}
)

func init() {
	// UnstableCodecTest: Non-deterministic codec WITH IsUnstable()=true
	// Expected behavior: Replay succeeds because unstable_serialization=true
	REGISTRY.AddDefinition(
		restate.NewObject("UnstableCodecTest").
			Handler("setAndAwait", restate.NewObjectHandler(
				func(ctx restate.ObjectContext, input string) (string, error) {
					// Set state with unstable codec that has IsUnstable()=true
					// PayloadOptions.unstable_serialization will be true
					restate.Set(ctx, "test-key", input, restate.WithCodec(UnstableCodec))

					// Create awakeable - causes suspension until completed externally
					awakeable := restate.Awakeable[string](ctx)

					// Store awakeable ID so we can retrieve it
					restate.Set(ctx, "awakeable-id", awakeable.Id())

					// Wait for awakeable - this suspends the invocation
					result, err := awakeable.Result()
					if err != nil {
						return "", err
					}

					// On replay: Set will serialize with DIFFERENT bytes (random nonce)
					// But because unstable_serialization=true, Restate skips byte comparison
					// and replay succeeds
					return "done: " + input + ", awakeable: " + result, nil
				})).
			Handler("getAwakeableId", restate.NewObjectSharedHandler(
				func(ctx restate.ObjectSharedContext, _ restate.Void) (string, error) {
					return restate.Get[string](ctx, "awakeable-id")
				})))

	// UnstableCodecFailTest: Non-deterministic codec WITHOUT IsUnstable()
	// Expected behavior: Replay FAILS with journal mismatch error
	REGISTRY.AddDefinition(
		restate.NewObject("UnstableCodecFailTest").
			Handler("setAndAwait", restate.NewObjectHandler(
				func(ctx restate.ObjectContext, input string) (string, error) {
					// Set state with unstable codec that does NOT have IsUnstable()
					// PayloadOptions.unstable_serialization will be false
					restate.Set(ctx, "test-key", input, restate.WithCodec(UnstableCodecNoFlag))

					// Create awakeable - causes suspension
					awakeable := restate.Awakeable[string](ctx)

					// Store awakeable ID
					restate.Set(ctx, "awakeable-id", awakeable.Id())

					// Wait for awakeable
					result, err := awakeable.Result()
					if err != nil {
						return "", err
					}

					// On replay: Set will serialize with DIFFERENT bytes
					// Because unstable_serialization=false, Restate compares bytes
					// and replay FAILS with journal mismatch error
					return "done: " + input + ", awakeable: " + result, nil
				})).
			Handler("getAwakeableId", restate.NewObjectSharedHandler(
				func(ctx restate.ObjectSharedContext, _ restate.Void) (string, error) {
					return restate.Get[string](ctx, "awakeable-id")
				})))
}
