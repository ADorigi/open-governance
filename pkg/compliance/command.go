package compliance

import (
	"context"
	"errors"
	"fmt"
	"os"

	config2 "github.com/kaytu-io/kaytu-util/pkg/config"
	"gitlab.com/keibiengine/keibi-engine/pkg/internal/httpserver"

	"github.com/spf13/cobra"
	"gitlab.com/keibiengine/keibi-engine/pkg/config"
	"go.uber.org/zap"
)

var (
	S3AccessKey    = os.Getenv("S3_ACCESS_KEY")
	S3AccessSecret = os.Getenv("S3_ACCESS_SECRET")
	S3Region       = os.Getenv("S3_REGION")
	S3Bucket       = os.Getenv("S3_BUCKET")
)

const (
	JobsQueueName    = "compliance-report-jobs-queue"
	ResultsQueueName = "compliance-report-results-queue"
)

type WorkerConfig struct {
	RabbitMQ              config.RabbitMQ
	ElasticSearch         config.ElasticSearch
	Kafka                 config.Kafka
	Compliance            config.KeibiService
	Onboard               config.KeibiService
	PrometheusPushAddress string
}

func WorkerCommand() *cobra.Command {
	var (
		id  string
		cnf WorkerConfig
	)
	config2.ReadFromEnv(&cnf, nil)

	cmd := &cobra.Command{
		PreRunE: func(cmd *cobra.Command, args []string) error {
			switch {
			case id == "":
				return errors.New("missing required flag 'id'")
			default:
				return nil
			}
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			logger, err := zap.NewProduction()
			if err != nil {
				return err
			}

			w, err := InitializeWorker(
				id,
				cnf,
				JobsQueueName,
				ResultsQueueName,
				logger,
				cnf.PrometheusPushAddress,
			)

			if err != nil {
				return err
			}

			defer w.Stop()

			return w.Run()
		},
	}

	cmd.Flags().StringVar(&id, "id", "", "The worker id")

	return cmd
}

type ServerConfig struct {
	ES         config.ElasticSearch
	PostgreSQL config.Postgres
	Scheduler  config.KeibiService
	Onboard    config.KeibiService
	Inventory  config.KeibiService
	Http       config.HttpServer
}

func ServerCommand() *cobra.Command {
	return &cobra.Command{
		RunE: func(cmd *cobra.Command, args []string) error {
			return startHttpServer(cmd.Context())
		},
	}
}

func startHttpServer(ctx context.Context) error {
	var conf ServerConfig
	config2.ReadFromEnv(&conf, nil)

	logger, err := zap.NewProduction()
	if err != nil {
		return fmt.Errorf("new logger: %w", err)
	}

	handler, err := InitializeHttpHandler(conf,
		S3AccessKey, S3AccessSecret, S3Region, S3Bucket,
		logger)
	if err != nil {
		return fmt.Errorf("init http handler: %w", err)
	}

	return httpserver.RegisterAndStart(logger, conf.Http.Address, handler)
}
