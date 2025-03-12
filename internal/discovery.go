package internal

import "github.com/restatedev/sdk-go/encoding"

type ProtocolMode string

const (
	ProtocolMode_BIDI_STREAM      ProtocolMode = "BIDI_STREAM"
	ProtocolMode_REQUEST_RESPONSE ProtocolMode = "REQUEST_RESPONSE"
)

type ServiceType string

const (
	ServiceType_VIRTUAL_OBJECT ServiceType = "VIRTUAL_OBJECT"
	ServiceType_SERVICE        ServiceType = "SERVICE"
	ServiceType_WORKFLOW       ServiceType = "WORKFLOW"
)

type ServiceHandlerType string

const (
	ServiceHandlerType_WORKFLOW  ServiceHandlerType = "WORKFLOW"
	ServiceHandlerType_EXCLUSIVE ServiceHandlerType = "EXCLUSIVE"
	ServiceHandlerType_SHARED    ServiceHandlerType = "SHARED"
)

type Handler struct {
	Name string `json:"name,omitempty"`
	// If unspecified, defaults to EXCLUSIVE for Virtual Object. This should be unset for Services.
	Ty            *ServiceHandlerType     `json:"ty,omitempty"`
	Input         *encoding.InputPayload  `json:"input,omitempty"`
	Output        *encoding.OutputPayload `json:"output,omitempty"`
	Metadata      map[string]string       `json:"metadata,omitempty"`
	Documentation string                  `json:"documentation,omitempty"`
}

type Service struct {
	Name          string            `json:"name"`
	Ty            ServiceType       `json:"ty"`
	Handlers      []Handler         `json:"handlers"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	Documentation string            `json:"documentation,omitempty"`
}

type Endpoint struct {
	ProtocolMode       ProtocolMode `json:"protocolMode"`
	MinProtocolVersion int32        `json:"minProtocolVersion"`
	MaxProtocolVersion int32        `json:"maxProtocolVersion"`
	Services           []Service    `json:"services"`
}
