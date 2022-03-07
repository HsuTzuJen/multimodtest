package clog

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"git.insea.io/hsucar/versiontest/version"
	"github.com/Shopify/sarama"
	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type KafkaConfig struct {
	KafkaAddr       string
	LogKafkaTopic   string
	EventKafkaTopic string
}

type Config struct {
	Path string
	*KafkaConfig
	Level string
}

var Logger *logrus.Logger
var kkhook *KafkaHook

func InitLog(config Config) error {
	if _, err := os.Stat(config.Path); os.IsNotExist(err) {
		dir := filepath.Dir(config.Path)
		if os.MkdirAll(dir, os.ModePerm) != nil {
			log.Printf("mkdir %v error\n", dir)
		}
	}

	absConfigPath, err := filepath.Abs(config.Path)
	if err != nil {
		log.Printf("filepath.Abs error=%v\n", err)
		return err
	}

	fileWriter, err := rotatelogs.New(
		absConfigPath+"-%Y%m%d",
		rotatelogs.WithLinkName(absConfigPath),
		rotatelogs.WithMaxAge(30*24*time.Hour),
	)
	if err != nil {
		log.Printf("rotatelogs.New error=%v\n", err)
		return err
	}

	logLevel, err := logrus.ParseLevel(config.Level)
	if err != nil {
		log.Printf("Parse log level error, level[%v]\n", config.Level)
		logLevel = logrus.DebugLevel
	}

	Logger = logrus.New()
	mw := io.MultiWriter(os.Stdout, fileWriter)
	Logger.Out = mw
	Logger.Formatter = &logrus.TextFormatter{}
	Logger.SetLevel(logLevel)
	Logger.AddHook(NewFileHook())
	Logger.AddHook(NewRequestIdHook())
	Logger.AddHook(NewTraceIdHook())

	if config.KafkaConfig != nil {
		if err := initKafkaHook(config.KafkaAddr, config.EventKafkaTopic, config.LogKafkaTopic, getHookLevels(logLevel)); err != nil {
			log.Printf("initKafkaHook error=%v\n", err)
			return err
		}
	}

	Logger.Infof("clog v1.3.0\n")
	Logger.Infof("mod git.insea.io/hsucar/versiontest %s", version.Ver)

	return nil
}

// LogEvent report log to elk
// needFilter --> true: filter; false:no filter
func LogEvent(data map[string]interface{}, needFilter bool) error {
	if kkhook == nil {
		// ignore event
		return nil
	}
	// sampling check event_type
	if _, ok := data["event_type"]; !ok {
		// not find event just return
		return nil
	}

	// filter event just in need
	if needFilter && (rand.Intn(10) != 0) {
		// not app_ccu event not streamer_ccu and sampling 1/10 event
		return nil
	}

	return kkhook.FireEvent(data)
}

func initKafkaHook(kafkaAddr, eventKafkaTopic, logKafkaTopic string, levels []logrus.Level) error {
	if kafkaAddr == "" {
		// ignore hooking.
		return nil
	}

	conf := sarama.NewConfig()
	conf.Producer.RequiredAcks = sarama.NoResponse
	conf.Producer.Return.Successes = false
	conf.Producer.Return.Errors = true

	kkCli, err := sarama.NewAsyncProducer(strings.Split(kafkaAddr, ";"), conf)
	if err != nil {
		Logger.Errorf("sarama.NewAsyncProducer error: %v, addr: %s", err, kafkaAddr)
		return err
	}
	processName := filepath.Base(os.Args[0])
	nodeName := os.Getenv("MY_NODE_NAME")
	hostName, err := os.Hostname()
	if err != nil {
		hostName = "localhost"
	}
	kkhook = NewKafakaHook(
		levels,
		&logrus.JSONFormatter{},
		kkCli,
		eventKafkaTopic,
		logKafkaTopic,
		processName,
		hostName,
		nodeName,
	)
	Logger.AddHook(kkhook)

	return nil
}

func getHookLevels(logLevel logrus.Level) []logrus.Level {
	res := make([]logrus.Level, 0)
	switch logLevel {
	case logrus.DebugLevel:
		res = append(res, logrus.DebugLevel)
		fallthrough
	case logrus.InfoLevel:
		res = append(res, logrus.InfoLevel)
		fallthrough
	case logrus.WarnLevel:
		res = append(res, logrus.WarnLevel)
		fallthrough
	case logrus.ErrorLevel:
		res = append(res, logrus.ErrorLevel)
		fallthrough
	case logrus.FatalLevel:
		res = append(res, logrus.FatalLevel)
		fallthrough
	case logrus.PanicLevel:
		res = append(res, logrus.PanicLevel)
		// should not put 'fallthrough' here
	default:
		res = []logrus.Level{
			logrus.DebugLevel,
			logrus.InfoLevel,
			logrus.WarnLevel,
			logrus.ErrorLevel,
			logrus.FatalLevel,
			logrus.PanicLevel,
		}
	}
	return res
}

func formatJSON(data map[string]interface{}) ([]byte, error) {
	b := &bytes.Buffer{}
	err := json.NewEncoder(b).Encode(data)
	if err != nil {
		return nil, fmt.Errorf("Failed to marshal fields to JSON, %v\n", err)
	}
	return b.Bytes(), nil
}

func OnReloadLogLevel(ctx context.Context, value interface{}) error {
	levelStr, ok := value.(string)
	if !ok {
		return errors.Errorf("type assert failed, value=%v", value)
	}
	logLevel, err := logrus.ParseLevel(levelStr)
	if err != nil {
		return errors.Wrapf(err, "parse log level falied, level=%v", logLevel)
	}
	Logger.SetLevel(logLevel)

	return nil
}

// GracefulStop shuts down the kafka producer kkCli of kkhook.
// It should be called in the main goroutine as the last cleaning logic.
// It should never be called before any cleaning logic since other
// cleaning logic may invoke the Logger.
func GracefulStop() {
	if kkhook != nil {
		kkhook.Close()
	}
}
