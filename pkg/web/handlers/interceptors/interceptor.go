package interceptors

import (
	"dkforest/pkg/web/handlers/interceptors/command"
)

type Interceptor interface {
	InterceptMsg(*command.Command)
}
