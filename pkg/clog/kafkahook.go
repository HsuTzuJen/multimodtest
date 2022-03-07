package clog

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/Shopify/sarama"
	"github.com/sirupsen/logrus"
)

const kkEventChanSize = 1024
const kkLogChanSize = 2048

type KafkaHook struct {
	levels      []logrus.Level
	Formatter   logrus.Formatter
	kkCli       sarama.AsyncProducer
	eventTopic  string
	logTopic    string
	eventChan   chan *sarama.ProducerMessage
	logChan     chan *sarama.ProducerMessage
	closing     chan struct{}
	dispatachWg sync.WaitGroup
	closeWg     sync.WaitGroup
	processName string
	hostName    string
	nodeName    string
}

// NewKafakaHook returns a new KafkaHook. Do not have a `handleKafkaSuccesses` for now
// since we set `conf.Producer.Return.Successes=false` in `initKafkaHook` and we only
// use KafkaHook in clog. If we set `conf.Producer.Return.Successes=true` in the futrue,
// we would have to have a `handleKafkaSuccesses` to handle `kkCli.Successes()`.
func NewKafakaHook(
	levels []logrus.Level,
	formatter logrus.Formatter,
	kkCli sarama.AsyncProducer,
	eventTopic string,
	logTopic string,
	processName string,
	hostName string,
	nodeName string,
) *KafkaHook {
	if kkCli == nil {
		panic("nil kkCli")
	}

	hook := &KafkaHook{
		levels:      levels,
		Formatter:   formatter,
		kkCli:       kkCli,
		eventTopic:  eventTopic,
		logTopic:    logTopic,
		eventChan:   make(chan *sarama.ProducerMessage, kkEventChanSize),
		logChan:     make(chan *sarama.ProducerMessage, kkLogChanSize),
		closing:     make(chan struct{}),
		processName: processName,
		hostName:    hostName,
		nodeName:    nodeName,
	}

	hook.closeWg.Add(1)
	go hook.handleKafkaError()
	hook.dispatachWg.Add(1)
	go hook.dispatchMessages()

	return hook
}

// EventInput is the input channel for the user to write messages to that they wish to send.
func (kh *KafkaHook) FireEvent(data map[string]interface{}) error {
	if _, ok := data["host"]; !ok {
		data["host"] = kh.hostName
	}

	if _, ok := data["process"]; !ok {
		data["process"] = kh.processName
	}

	if _, ok := data["nodename"]; !ok {
		data["nodename"] = kh.nodeName
	}

	var (
		bt  []byte
		err error
	)
	if bt, err = formatJSON(data); err != nil {
		Logger.Errorf("log event error: %v", err)
		return err
	}
	Logger.Debugf("sending log event: %v", string(bt))

	select {
	case <-kh.closing: // This may happen if this call is after kh.Close()
		fmt.Printf("kkCli started to close, message not produced, msg=%v\n", string(bt))
		return nil
	default:
	}

	select { // log should not block our business.
	case kh.eventChan <- &sarama.ProducerMessage{Topic: kh.eventTopic, Value: sarama.ByteEncoder(bt)}:

	default:
		fmt.Printf("kkEventChan is full. message not produced, msg=%v\n", string(bt))
	}

	return nil
}

func (kh *KafkaHook) Fire(entry *logrus.Entry) error {
	if kh.kkCli == nil {
		return nil
	}

	var (
		bt  []byte
		err error
	)

	if bt, err = entry.Time.MarshalBinary(); err != nil {
		return err
	}

	if _, ok := entry.Data["nodename"]; !ok {
		entry.Data["nodename"] = kh.nodeName
		defer delete(entry.Data, "nodename")
	}

	if _, ok := entry.Data["host"]; !ok {
		entry.Data["host"] = kh.hostName
		defer delete(entry.Data, "host")
	}

	if _, ok := entry.Data["process"]; !ok {
		entry.Data["process"] = kh.processName
		defer delete(entry.Data, "process")
	}

	if bt, err = kh.Formatter.Format(entry); err != nil {
		return err
	}

	select {
	case <-kh.closing: // This may happen if this call is after kh.Close()
		fmt.Printf("kkCli started to close, message not produced, msg=%v\n", string(bt))
		return nil
	default:
	}

	select { // log should not block our business
	case kh.logChan <- &sarama.ProducerMessage{Topic: kh.logTopic, Value: sarama.ByteEncoder(bt)}:

	default:
		fmt.Printf("kkLogChan is full. message not produced, msg=%v\n", string(bt))
	}

	return nil
}

func (kh *KafkaHook) Levels() []logrus.Level {
	return kh.levels
}

func (kh *KafkaHook) Close() {
	startTime := time.Now()

	close(kh.closing)

	// this wg is done when `dispatchMessages` returns
	kh.dispatachWg.Wait()

	fmt.Printf("kkCli starts closing, undrained msg size: log=%v, event=%v", len(kh.logChan), len(kh.eventChan))
	kh.kkCli.AsyncClose()

	// this wg is done when `handleKafkaError` returns
	kh.closeWg.Wait()

	fmt.Printf("kkCli closed: duration=%v", time.Since(startTime))
}

func (kh *KafkaHook) handleKafkaError() {
	// kh.kkCli.Errors() would be closed after `kh.kkCli.AsyncClose`
	// is invoked and the underlying `shutdown` is completed.
	defer kh.closeWg.Done()

	for err := range kh.kkCli.Errors() {
		if err != nil {
			var msgStr string
			if err.Msg.Value != nil {
				msgValBytes, _ := err.Msg.Value.Encode()
				msgStr = string(msgValBytes)
			}
			fmt.Fprintf(os.Stderr, "Kafka client topic: %s, error: %v, log: %v\n", err.Msg.Topic, err.Err, msgStr)
		}
	}
}

func (kh *KafkaHook) dispatchMessages() {
	defer kh.dispatachWg.Done()

	kkCliInput := kh.kkCli.Input()
	for {
		select {
		case <-kh.closing:
			return
		case msg := <-kh.eventChan:
			kkCliInput <- msg
		case msg := <-kh.logChan:
			kkCliInput <- msg
		}
	}
}
