package ingress

type Invocation struct {
	Id     string `json:"invocationId"`
	Status string `json:"status"`
	Error  error  `json:"-"`
}
