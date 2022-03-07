package clog

import (
	"git.insea.io/booyah/server/kakarot/pkg/requestid"
	"github.com/sirupsen/logrus"
)

const FieldKeyRequestId = "requestId"

type RequestIdHook struct {
	Field  string
	levels []logrus.Level
}

func NewRequestIdHook() *RequestIdHook {
	return &RequestIdHook{
		Field:  FieldKeyRequestId,
		levels: logrus.AllLevels,
	}
}

func (hook *RequestIdHook) Levels() []logrus.Level {
	return hook.levels
}

func (hook *RequestIdHook) Fire(entry *logrus.Entry) error {
	requestId := requestid.FromContext(entry.Context)
	if requestId != "" {
		entry.Data[hook.Field] = requestId
	}

	return nil
}
