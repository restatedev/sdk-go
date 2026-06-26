package restate

import (
	rand2 "math/rand/v2"

	"github.com/google/uuid"
)

// Rand returns a random source which will give deterministic results for a given invocation
//
// This rand instance is not safe for use inside .Run()
func Rand(ctx Context) *rand2.Rand {
	return ctx.inner().RandInstance()
}

// UUID returns a random UUID seeded deterministically for a given invocation.
//
// This method should not be used inside .Run()
func UUID(ctx Context) uuid.UUID {
	return ctx.inner().RandUUID()
}

// RandSource returns a random source to be used with math/rand/v2 implementations.
//
// To create a random implementation, use `rand2.New(RandSource(ctx))`
//
// This source instance is not safe for use inside .Run()
func RandSource(ctx Context) rand2.Source {
	return ctx.inner().RandSource()
}
