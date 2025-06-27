package ingress

type restateError struct {
	Message     string `json:"message"`
	Code        int    `json:"code,omitempty"`
	Description string `json:"description,omitempty"`
	Stacktrace  string `json:"stacktrace,omitempty"`
}

type GenericError struct {
	*restateError
}

type InvocationNotFoundError struct {
	*restateError
}

type InvocationNotReadyError struct {
	*restateError
}

func (e restateError) Error() string {
	return e.Message
}

func newGenericError(err *restateError) *GenericError {
	return &GenericError{
		restateError: err,
	}
}

func newInvocationNotFoundError(err *restateError) *InvocationNotFoundError {
	return &InvocationNotFoundError{
		restateError: err,
	}
}

func newInvocationNotReadyError(err *restateError) *InvocationNotReadyError {
	return &InvocationNotReadyError{
		restateError: err,
	}
}
