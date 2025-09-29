package record

import (
	"context"
)

type contextKey string

func (r *Record) SetContextValue(key string, value string) {
	if r.Context == nil {
		r.Context = context.Background()
	}
	r.Context = context.WithValue(r.Context, contextKey(key), string(value))
}

func (r *Record) GetContextValue(key string) (string, bool) {

	if ctx := r.Context; ctx != nil {
		if v := ctx.Value(contextKey(key)); v != nil {
			vString, ok := v.(string)
			return vString, ok
		}
	}

	return ``, false

}
