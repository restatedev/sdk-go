package restate

import (
	"iter"

	"github.com/restatedev/sdk-go/internal/restatecontext"
)

// Future is a marker interface for futures.
type Future = restatecontext.Future

// WaitFirst waits for the first Future to complete among the provided Futures and returns it.
// If the invocation is canceled, a cancellation error is returned.
//
// Example:
//
//	func MyHandler(ctx restate.Context, input string) (string, error) {
//		fut1 := restate.Service[string](ctx, "service1", "method1").RequestFuture(input)
//		fut2 := restate.After(ctx, 5 * time.Second)
//		fut3 := restate.Service[string](ctx, "service2", "method2").RequestFuture(input)
//
//		firstComplete, err := restate.WaitFirst(ctx, fut1, fut2, fut3)
//		if err != nil {
//			return "", err
//		}
//		// Handle the first completed future
//		switch firstComplete {
//		case fut1:
//			return fut1.Response()
//		case fut2:
//			return "", fmt.Errorf("timeout")
//		case fut3:
//			return fut3.Response()
//		default:
//			return "", fmt.Errorf("unknown future")
//		}
//	}
func WaitFirst(ctx Context, futs ...Future) (resultFut Future, cancellationError TerminalError) {
	set := WaitIter(ctx, futs...)
	set.Next()
	resultFut, cancellationError = set.Value(), set.Err()
	return
}

// Wait returns an iterator that yields Futures as they complete in order of completion.
// The iterator continues until all Futures have completed or a cancellation error occurs.
// If a cancellation error occurs, it is yielded as the final element with a nil Future.
//
// Example:
//
//	func MyHandler(ctx restate.Context, input string) ([]string, error) {
//		fut1 := restate.Service[string](ctx, "service1", "method1").RequestFuture(input)
//		fut2 := restate.Service[string](ctx, "service2", "method2").RequestFuture(input)
//		fut3 := restate.Service[string](ctx, "service3", "method3").RequestFuture(input)
//
//		results := []string{}
//		for fut, err := range restate.Wait(ctx, fut1, fut2, fut3) {
//			if err != nil {
//				return nil, err
//			}
//			result, err := fut.(restate.ResponseFuture[string]).Response()
//			if err != nil {
//				return nil, err
//			}
//			results = append(results, result)
//		}
//		return results, nil
//	}
func Wait(ctx Context, futs ...Future) iter.Seq2[Future, TerminalError] {
	set := WaitIter(ctx, futs...)

	return func(yield func(Future, TerminalError) bool) {
		for set.Next() {
			value := set.Value()
			if value == nil {
				break
			}
			if !yield(value, nil) {
				return
			}
		}
		if err := set.Err(); err != nil {
			yield(nil, err)
		}
	}
}

// WaitIter returns an iterator that allows manual control over waiting for multiple Futures to complete.
// This is the low-level primitive that WaitFirst and Wait are built on top of.
// Call Next() to wait for the next Future to complete, then use Value() to retrieve it and Err() to check for errors.
//
// Example:
//
//	func MyHandler(ctx restate.Context, input string) (string, error) {
//		fut1 := restate.Service[string](ctx, "service1", "method1").RequestFuture(input)
//		fut2 := restate.Service[string](ctx, "service2", "method2").RequestFuture(input)
//
//		iter := restate.WaitIter(ctx, fut1, fut2)
//		for iter.Next() {
//			fut := iter.Value()
//			// Process each future as it completes
//			if fut == fut1 {
//				result, _ := fut1.Response()
//				fmt.Printf("fut1 completed with: %s\n", result)
//			}
//		}
//		if err := iter.Err(); err != nil {
//			return "", err
//		}
//		return "all done", nil
//	}
func WaitIter(ctx Context, futs ...Future) WaitIterator {
	return ctx.inner().WaitIter(futs...)
}

// WaitIterator is an iterator over a list of blocking Restate operations that are running
// in the background. See WaitIter for more details.
type WaitIterator = restatecontext.WaitIterator
