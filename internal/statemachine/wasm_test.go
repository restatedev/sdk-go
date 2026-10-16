package statemachine

import (
	"context"
	"testing"
)

func TestSharedCoreIsCompiledOnFirstUseNotAtInit(t *testing.T) {
	if _sharedCoreMod != nil || _wazeroRuntime != nil {
		t.Fatal("the shared core was compiled at package init")
	}

	ctx := context.Background()
	first, err := NewCore(ctx)
	if err != nil {
		t.Fatalf("NewCore: %v", err)
	}
	compiled := _sharedCoreMod
	if compiled == nil {
		t.Fatal("NewCore did not compile the shared core")
	}

	if _, err := NewCore(ctx); err != nil {
		t.Fatalf("second NewCore: %v", err)
	}
	if _sharedCoreMod != compiled {
		t.Fatal("the shared core was compiled twice")
	}
	_ = first
}
