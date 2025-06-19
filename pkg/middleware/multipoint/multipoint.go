package multipoint

import (
	"context"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
)

type validator interface {
	Validate() error
}

type tokenKey struct{}

type options struct {
}

type Option func(o *options)

func Server(opts ...Option) middleware.Middleware {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (reply interface{}, err error) {
			if _, ok := transport.FromServerContext(ctx); ok {
				if v, ok := req.(validator); ok {
					if err := v.Validate(); err != nil {
						return nil, err
					}
				}
			}
			return handler(ctx, req)
		}
	}
}
