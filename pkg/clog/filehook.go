package clog

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/sirupsen/logrus"
)

const (
	FieldKeyFile       = "file"
	FileHookSkip       = 5
	FileHookSkipPrefix = "logrus"
)

type FileHook struct {
	Field  string
	levels []logrus.Level
}

func NewFileHook() *FileHook {
	return &FileHook{
		Field:  FieldKeyFile,
		levels: logrus.AllLevels,
	}
}

func (hook *FileHook) Levels() []logrus.Level {
	return hook.levels
}

func (hook *FileHook) Fire(entry *logrus.Entry) error {
	entry.Data[hook.Field] = findCaller()

	return nil
}

func findCaller() string {
	var (
		file string
		line int
	)

	for i := 0; i < 10; i++ {
		file, line = getCaller(FileHookSkip + i)
		if !strings.HasPrefix(file, FileHookSkipPrefix) {
			break
		}
	}

	return fmt.Sprintf("%s:%d", file, line)
}

func getCaller(skip int) (string, int) {
	// runtime.Caller() will return file like:
	// $PROJECT_PATH/kakarot/common/clog/filehook.go
	// $GOPATH/pkg/mod/git.insea.io/booyah/server/logrus@v1.4.6/logger.go
	_, file, line, ok := runtime.Caller(skip)
	if !ok {
		return "", 0
	}

	n := 0
	for i := len(file) - 1; i > 0; i-- {
		if file[i] == '/' {
			n++
			if n == 2 {
				file = file[i+1:]
				break
			}
		}
	}

	return file, line
}
