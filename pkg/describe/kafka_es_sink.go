package describe

import (
	"fmt"
	"time"

	confluent_kafka "github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/kaytu-io/kaytu-util/pkg/kafka"
	"github.com/kaytu-io/kaytu-util/pkg/keibi-es-sdk"
	"go.uber.org/zap"
)

type esResource struct {
	index string
	id    string
	body  []byte
}

type KafkaEsSink struct {
	logger        *zap.Logger
	kafkaConsumer *confluent_kafka.Consumer
	esClient      keibi.Client
	commitChan    chan *confluent_kafka.Message
	esSinkChan    chan *confluent_kafka.Message
	esSinkBuffer  []*confluent_kafka.Message
}

func NewKafkaEsSink(logger *zap.Logger, kafkaConsumer *confluent_kafka.Consumer, esClient keibi.Client) *KafkaEsSink {
	return &KafkaEsSink{
		logger:        logger,
		kafkaConsumer: kafkaConsumer,
		esClient:      esClient,
		commitChan:    make(chan *confluent_kafka.Message),
		esSinkChan:    make(chan *confluent_kafka.Message),
		esSinkBuffer:  make([]*confluent_kafka.Message, 0, 100),
	}
}

func (s *KafkaEsSink) Run() {
	EnsureRunGoroutin(func() {
		s.runKafkaRead()
	})
	EnsureRunGoroutin(func() {
		s.runKafkaCommit()
	})
	EnsureRunGoroutin(func() {
		s.runElasticSearchSink()
	})
}

func (s *KafkaEsSink) runElasticSearchSink() {
	for {
		select {
		case resource := <-s.esSinkChan:
			s.esSinkBuffer = append(s.esSinkBuffer, resource)
			if len(s.esSinkBuffer) > 1000 {
				s.flushESSinkBuffer()
			}
		case <-time.After(30 * time.Second):
			s.flushESSinkBuffer()
		}
	}
}

func (s *KafkaEsSink) runKafkaRead() {
	for {
		ev := s.kafkaConsumer.Poll(100)
		if ev == nil {
			continue
		}
		switch e := ev.(type) {
		case *confluent_kafka.Message:
			s.esSinkChan <- e
		case confluent_kafka.Error:
			s.logger.Error("Consumer Kafka error", zap.Error(e))
		default:
			s.logger.Info("Ignored kafka event from topic", zap.Any("event", e))
		}
	}
}

func (s *KafkaEsSink) runKafkaCommit() {
	for msg := range s.commitChan {
		_, err := s.kafkaConsumer.CommitMessage(msg)
		if err != nil {
			s.logger.Error("Failed to commit kafka message", zap.Error(err))
		}
	}
}

func (s *KafkaEsSink) flushESSinkBuffer() {
	if len(s.esSinkBuffer) == 0 {
		return
	}

	//esMsgs := make([]*esResource, 0, len(s.esSinkBuffer))
	//for _, msg := range s.esSinkBuffer {
	//	resource, err := newEsResourceFromKafkaMessage(msg)
	//	if err != nil || resourceEvent == nil {
	//		s.logger.Error("Failed to parse kafka message", zap.Error(err))
	//		continue
	//	}
	//	esMsgs = append(esMsgs, resource)
	//}
	//TODO Send to ES

	for _, resourceEvent := range s.esSinkBuffer {
		s.commitChan <- resourceEvent
	}

	s.esSinkBuffer = make([]*confluent_kafka.Message, 0, 100)
}

func newEsResourceFromKafkaMessage(msg *confluent_kafka.Message) (*esResource, error) {
	var resource esResource
	index := ""
	for _, h := range msg.Headers {
		if h.Key == kafka.EsIndexHeader {
			index = string(h.Value)
		}
	}
	if index == "" {
		return nil, fmt.Errorf("missing index header")
	}
	resource.index = index
	resource.id = string(msg.Key)
	resource.body = msg.Value

	return &resource, nil
}
