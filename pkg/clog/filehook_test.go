package clog

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"runtime"
	"strings"
	"testing"

	assert "github.com/stretchr/testify/require"
)

func TestLoggerCaller(t *testing.T) {
	logFile := "./log/test.log"

	err := os.RemoveAll("./log")
	if err != nil {
		panic(err)
	}

	err = InitLog(Config{Path: logFile, Level: "debug"})
	if err != nil {
		panic(err)
	}

	_, file, line, ok := runtime.Caller(0)
	Logger.Errorf("test log 1")
	Logger.WithContext(context.Background()).Errorf("test log 2")

	if !ok {
		panic(errors.New("unable to find caller"))
	}

	fileName := file[strings.LastIndex(file, "/")+1:]
	line1 := line + 1
	line2 := line + 2

	c, err := ioutil.ReadFile(logFile)

	if err != nil {
		panic(err)
	}

	lines := strings.Split(strings.Trim(string(c), "\n"), "\n")

	assert.Equal(t, 2, len(lines))

	assert.Contains(t, lines[0], fmt.Sprintf("%v:%v", fileName, line1))
	assert.Contains(t, lines[1], fmt.Sprintf("%v:%v", fileName, line2))
}
