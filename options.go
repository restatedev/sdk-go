package restate

import (
	"github.com/restatedev/sdk-go/internal/errors"
	"github.com/restatedev/sdk-go/internal/options"
)

// Shared options that apply across more than one API section live here. Operation-specific
// options live alongside their API (e.g. rpc.go, run.go, state.go, codec.go, invocation_options.go).

// WithMetadataMap adds the given metadata. It applies anywhere metadata is accepted:
// service/handler definitions (shown in the Admin API) and [ToTerminalError]. Multiple
// metadata options merge.
func WithMetadataMap(metadata map[string]string) errors.MetadataOption {
	return errors.WithMetadata(metadata)
}

// WithMetadata adds the given key/value as metadata. It applies anywhere metadata is
// accepted: service/handler definitions (shown in the Admin API) and [ToTerminalError].
// Multiple metadata options merge.
func WithMetadata(metadataKey string, metadataValue string) errors.MetadataOption {
	return errors.WithMetadata(map[string]string{metadataKey: metadataValue})
}

type withName struct {
	name string
}

var _ options.RunOption = withName{}
var _ options.SleepOption = withName{}

func (w withName) BeforeRun(opts *options.RunOptions) {
	opts.Name = w.name
}

func (w withName) BeforeSleep(opts *options.SleepOptions) {
	opts.Name = w.name
}

// WithName sets the operation name, shown in the UI and other Restate observability tools.
func WithName(name string) withName {
	return withName{name}
}
