package restatecontext

import (
	"github.com/restatedev/sdk-go/internal/errors"
	pbinternal "github.com/restatedev/sdk-go/internal/generated"
)

func newFailureFromError(err error) *pbinternal.Failure {
	failure := pbinternal.Failure{}
	terminalError := errors.ToTerminalError(err)
	if terminalError == nil {
		panic("expecting err to be non-nil")
	}
	failure.SetCode(uint32(terminalError.Code()))
	failure.SetMessage(terminalError.Message())
	failure.SetMetadata(metadataToHeaders(terminalError.Metadata()))
	return &failure
}

func metadataToHeaders(metadata map[string]string) []*pbinternal.Header {
	if len(metadata) == 0 {
		return nil
	}
	headers := make([]*pbinternal.Header, 0, len(metadata))
	for k, v := range metadata {
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
