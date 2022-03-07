package clog

import (
	"context"

	"github.com/opentracing/opentracing-go"
	"github.com/sirupsen/logrus"
	"github.com/uber/jaeger-client-go"
)

const FieldKeyTraceId = "traceId"

type TraceIdHook struct {
	Field  string
	levels []logrus.Level
}

func NewTraceIdHook() *TraceIdHook {
	return &TraceIdHook{
		Field:  FieldKeyTraceId,
		levels: logrus.AllLevels,
	}
}

func (hook *TraceIdHook) Levels() []logrus.Level {
	return hook.levels
}

func (hook *TraceIdHook) Fire(entry *logrus.Entry) error {
	traceId := sampledTraceIdFromContext(entry.Context)
	if traceId != "" {
		entry.Data[hook.Field] = traceId
	}

	return nil
}

// Only record sampled trace id in log record
func sampledTraceIdFromContext(ctx context.Context) string {
	if ctx != nil {
		if span := opentracing.SpanFromContext(ctx); span != nil {
			if sc, ok := span.Context().(jaeger.SpanContext); ok && sc.IsSampled() {
				return sc.TraceID().String()
			}
		}
	}
	return ""
}
