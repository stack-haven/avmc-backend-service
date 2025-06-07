package localize

import (
	"context"

	"github.com/BurntSushi/toml"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/go-kratos/kratos/v2/transport/http"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

type validator interface {
	Validate() error
}

type localizerKey struct{}

type options struct {
	defaultLanguage language.Tag
	messagePath     string
}

type Option func(o *options)

func WithMessagePath(path string) Option {
	return func(o *options) {
		o.messagePath = path
	}
}

func WithDefaultLanguage(language language.Tag) Option {
	return func(o *options) {
		o.defaultLanguage = language
	}
}

func I18N(opts ...Option) middleware.Middleware {
	o := &options{
		defaultLanguage: language.English,
		messagePath:     "../../i18n/active.es.toml",
	}
	for _, opt := range opts {
		opt(o)
	}
	bundle := i18n.NewBundle(o.defaultLanguage)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)
	bundle.MustLoadMessageFile(o.messagePath)

	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (reply interface{}, err error) {
			lang := o.defaultLanguage.String()
			if tr, ok := transport.FromServerContext(ctx); ok {
				if ht, ok := tr.(http.Transporter); ok {
					lang = ht.Request().FormValue("lang")
				}
				accept := tr.RequestHeader().Get("Accept-Language")
				localizer := i18n.NewLocalizer(bundle, lang, accept)
				ctx = context.WithValue(ctx, localizerKey{}, localizer)
			}
			// 请求验证（处理请求验证翻译）
			// if v, ok := req.(validator); ok {
			// 	if err := v.Validate(); err != nil {
			// 		return nil, errors.BadRequest("VALIDATOR", err.Error()).WithCause(err)
			// 	}
			// }
			return handler(ctx, req)
		}
	}
}

func FromContext(ctx context.Context) *i18n.Localizer {
	return ctx.Value(localizerKey{}).(*i18n.Localizer)
}
