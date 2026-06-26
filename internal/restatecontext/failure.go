package restatecontext

import (
	"github.com/restatedev/sdk-go/internal/errors"
	pbinternal "github.com/restatedev/sdk-go/internal/generated"
	"github.com/restatedev/sdk-go/internal/stringmap"
)

func newFailureFromError(err error) *pbinternal.TerminalFailure {
	failure := pbinternal.TerminalFailure{}
	terminalError := errors.ToTerminalError(err)
	if terminalError == nil {
		panic("expecting err to be non-nil")
	}
	failure.SetCode(uint32(terminalError.Code()))
	failure.SetMessage(terminalError.Message())
	failure.SetMetadata(metadataToHeaders(terminalError.Metadata()))
	return &failure
}

// metadataToHeaders converts metadata to wire headers. It iterates in the Map's
// deterministic (key-sorted) order: the ordering must be stable before it reaches the
// wasm layer.
func metadataToHeaders(metadata stringmap.Map) []*pbinternal.Header {
	if metadata == nil {
		return nil
	}
	var headers []*pbinternal.Header
	for k, v := range metadata.Iter() {
		header := pbinternal.Header{}
		header.SetKey(k)
		header.SetValue(v)
		headers = append(headers, &header)
	}
	return headers
}

func metadataFromHeaders(headers []*pbinternal.Header) map[string]string {
	if len(headers) == 0 {
		return nil
	}
	metadata := make(map[string]string, len(headers))
	for _, header := range headers {
		metadata[header.GetKey()] = header.GetValue()
	}
	return metadata
}
