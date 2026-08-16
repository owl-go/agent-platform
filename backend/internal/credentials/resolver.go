package credentials

import "context"

type Binding struct {
	Ref     string `json:"ref"`
	Purpose string `json:"purpose"`
}

type Resolver interface {
	Resolve(context.Context, string, []Binding) (Request, error)
}
