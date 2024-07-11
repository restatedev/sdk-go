package futures_test

import (
	"context"
	"fmt"
	"testing"

	restate "github.com/restatedev/sdk-go"
	"github.com/restatedev/sdk-go/internal/futures"
)

type fakeContext struct {
	restate.Context
}

func (f *fakeContext) Awakeable() restate.Awakeable[[]byte] {
	return futures.NewAwakeable(context.TODO(), nil, nil, 0)
}

var _ restate.Context = (*fakeContext)(nil)

func TestSelect(t *testing.T) {
	after := futures.NewAfter(context.TODO(), nil, 0)
	awakeableOne := futures.NewAwakeable(context.TODO(), nil, nil, 0)
	awakeableTwo := restate.AwakeableAs[string](&fakeContext{})
	responseFut := futures.NewResponseFuture(context.TODO(), nil, 0)

	// one-off (race)
	selector := futures.Select(context.TODO(), after, awakeableOne, awakeableTwo, responseFut)
	if !selector.Select() {
		t.Fatal(selector.Err())
	}
	switch selector.Result() {
	case after:
		t.Log("after won")
	case awakeableOne:
		t.Log("awakeable one won")
	case awakeableTwo:
		t.Log("awakeable two won")
	case responseFut:
		t.Log("response won")
	}

	// or as a loop (all or any)
	selector = futures.Select(context.TODO(), after, awakeableOne, awakeableTwo, responseFut)
	for selector.Select() {
		switch selector.Result() {
		case after:
			t.Log("after")
		case awakeableOne:
			t.Log("awakeable one")
		case awakeableTwo:
			t.Log("awakeable two")
		case responseFut:
			t.Log("response")
		}
	}

	if selector.Err() != nil {
		t.Fatal(selector.Err())
	}

}

func TestFailedSelect(t *testing.T) {
	err := fmt.Errorf("oops")
	failedResponseFut := futures.NewFailedResponseFuture(context.TODO(), err)
	selector := futures.Select(context.TODO(), failedResponseFut)
	if selector.Select() {
		t.Fatal("Select() should return false immediately")
	}
	if selector.Err() == nil {
		t.Fatal("Err() should return an error")
	}
	if selector.Err() != err {
		t.Fatalf("Err() returned an unexpected err: %v", err)
	}
}
