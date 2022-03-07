package requestid

import "context"

type requestIdKey struct{}

var activeRequestIdKey = requestIdKey{}

func ContextWithRequestId(ctx context.Context, requestId string) context.Context {
	return context.WithValue(ctx, activeRequestIdKey, requestId)
}

func FromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}

	if val, ok := ctx.Value(activeRequestIdKey).(string); ok {
		return val
	} else {
		return ""
	}
}
