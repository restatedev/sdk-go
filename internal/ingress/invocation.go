package ingress

type Invocation struct {
	Id     string `json:"id"`
	Status string `json:"status"`
	Error  error  `json:"-"`
}
