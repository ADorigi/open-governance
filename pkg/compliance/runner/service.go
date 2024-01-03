package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	kafka2 "github.com/confluentinc/confluent-kafka-go/v2/kafka"
	complianceClient "github.com/kaytu-io/kaytu-engine/pkg/compliance/client"
	inventoryClient "github.com/kaytu-io/kaytu-engine/pkg/inventory/client"
	"github.com/kaytu-io/kaytu-engine/pkg/jq"
	onboardClient "github.com/kaytu-io/kaytu-engine/pkg/onboard/client"
	"github.com/kaytu-io/kaytu-util/pkg/config"
	"github.com/kaytu-io/kaytu-util/pkg/kaytu-es-sdk"
	"github.com/kaytu-io/kaytu-util/pkg/source"
	"github.com/kaytu-io/kaytu-util/pkg/steampipe"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"
)

const (
	JobQueueTopic    = "compliance-runner-job-queue"
	ResultQueueTopic = "compliance-runner-job-result"
	ConsumerGroup    = "compliance-runner"

	// Jobs are processed in goroutines, by increasing this value
	// you will get more concurrent jobs at the same time and by decreasing
	// it they will more likely timeout.
	// Please note that, the value is set on the NATS produce context.
	JobProcessingTimeout = 10 * time.Second
)

type Config struct {
	ElasticSearch         config.ElasticSearch
	NATS                  config.NATS
	Compliance            config.KaytuService
	Onboard               config.KaytuService
	Inventory             config.KaytuService
	Steampipe             config.Postgres
	PrometheusPushAddress string
}

type Worker struct {
	config           Config
	logger           *zap.Logger
	steampipeConn    *steampipe.Database
	esClient         kaytu.Client
	jq               *jq.JobQueue
	complianceClient complianceClient.ComplianceServiceClient
	onboardClient    onboardClient.OnboardServiceClient
	inventoryClient  inventoryClient.InventoryServiceClient
}

func InitializeNewWorker(
	config Config,
	logger *zap.Logger,
	prometheusPushAddress string,
) (*Worker, error) {
	err := steampipe.PopulateSteampipeConfig(config.ElasticSearch, source.CloudAWS)
	if err != nil {
		return nil, err
	}
	err = steampipe.PopulateSteampipeConfig(config.ElasticSearch, source.CloudAzure)
	if err != nil {
		return nil, err
	}

	steampipeConn, err := steampipe.StartSteampipeServiceAndGetConnection(logger)
	if err != nil {
		return nil, err
	}

	esClient, err := kaytu.NewClient(kaytu.ClientConfig{
		Addresses:     []string{config.ElasticSearch.Address},
		Username:      &config.ElasticSearch.Username,
		Password:      &config.ElasticSearch.Password,
		IsOpenSearch:  &config.ElasticSearch.IsOpenSearch,
		AwsRegion:     &config.ElasticSearch.AwsRegion,
		AssumeRoleArn: &config.ElasticSearch.AssumeRoleArn,
	})
	if err != nil {
		return nil, err
	}

	jq, err := jq.New(config.NATS.URL, logger)
	if err != nil {
		return nil, err
	}

	w := &Worker{
		config:           config,
		logger:           logger,
		steampipeConn:    steampipeConn,
		esClient:         esClient,
		jq:               jq,
		complianceClient: complianceClient.NewComplianceClient(config.Compliance.BaseURL),
		onboardClient:    onboardClient.NewOnboardServiceClient(config.Onboard.BaseURL),
		inventoryClient:  inventoryClient.NewInventoryServiceClient(config.Inventory.BaseURL),
	}

	return w, nil
}

// Run should be called in another goroutine. It runs a NATS consumer and it will close it
// when the given context is closed.
func (w *Worker) Run(ctx context.Context) error {
	w.logger.Info("starting to consume")

	consumeCtx, err := w.jq.Consume(ctx, "compliance", "", []string{JobQueueTopic}, ConsumerGroup, func(msg jetstream.Msg) {
		w.logger.Info("received a new job")

		go func() {
			ctx, done := context.WithTimeout(context.Background(), JobProcessingTimeout)
			defer done()

			commit, requeue, err := w.ProcessMessage(ctx, msg)
			if err != nil {
				w.logger.Error("failed to process message", zap.Error(err))
			}

			if requeue {
				if err := msg.Nak(); err != nil {
					w.logger.Error("failed to send a not ack message", zap.Error(err))
				}
			}

			if commit {
				w.logger.Info("committing")
				if err := msg.Ack(); err != nil {
					w.logger.Error("failed to send an ack message", zap.Error(err))
				}
			}

			w.logger.Info("processing a job completed")
		}()
	})
	if err != nil {
		return err
	}

	w.logger.Info("consuming")

	for {
		select {
		case <-ctx.Done():
			consumeCtx.Stop()
		default:
		}
	}
}

func (w *Worker) ProcessMessage(ctx context.Context, msg jetstream.Msg) (commit bool, requeue bool, err error) {
	startTime := time.Now()

	var job Job
	err = json.Unmarshal(msg.Data(), &job)
	if err != nil {
		return true, false, err
	}

	result := JobResult{
		Job:               job,
		StartedAt:         startTime,
		Status:            ComplianceRunnerSucceeded,
		Error:             "",
		TotalFindingCount: nil,
	}
	defer func() {
		if err != nil {
			result.Error = err.Error()
			result.Status = ComplianceRunnerFailed
		}

		resultJson, err := json.Marshal(result)
		if err != nil {
			w.logger.Error("failed to create job result json", zap.Error(err))
			return
		}

		fmt.Sprintf("job-result-%d", job.ID)
		if err := w.jq.Produce(ctx, ResultQueueTopic, resultJson); err != nil {
			w.logger.Error("failed to publish job result", zap.String("jobResult", string(resultJson)), zap.Error(err))
		}
	}()

	w.logger.Info("running job", zap.ByteString("job", msg.Data()))
	totalFindingCount, err := w.RunJob(ctx, job)
	if err != nil {
		return true, false, err
	}

	result.TotalFindingCount = &totalFindingCount

	return true, false, nil
}

func (w *Worker) Stop() error {
	w.steampipeConn.Conn().Close()
	steampipe.StopSteampipeService(w.logger)
	return nil
}

func newKafkaProducer(kafkaServers []string) (*kafka2.Producer, error) {
	return kafka2.NewProducer(&kafka2.ConfigMap{
		"bootstrap.servers": strings.Join(kafkaServers, ","),
		"acks":              "all",
		"retries":           3,
		"linger.ms":         1,
		"batch.size":        1000000,
		"compression.type":  "lz4",
	})
}
