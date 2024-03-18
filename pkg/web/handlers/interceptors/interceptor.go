package interceptors

import (
	"errors"
	"dkforest/pkg/web/handlers/interceptors/command"
)

type Interceptor interface {
	Intercept(i interface{}) error
}

func (i *InterceptorImpl) Intercept(input interface{}) error {
	switch input.(type) {
	case *command.Command:
		i.InterceptMsg(input.(*command.Command))
	default:
		return errors.New("unsupported input type")
	}
	return nil
}

type InterceptorImpl struct {
	InterceptMsg func(*command.Command)
}
