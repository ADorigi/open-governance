package tasks

import (
	"context"

	"github.com/kaytu-io/open-governance/pkg/healthcheck"

	"github.com/adorigi/workerpool"
	"go.uber.org/zap"
)

type PostgresCheckTask struct {
	workerpool.TaskProperties
	logger *zap.Logger
}

func NewPostgresCheckTask(logger *zap.Logger, taskProperties workerpool.TaskProperties) *PostgresCheckTask {
	return &PostgresCheckTask{
		logger:         logger,
		TaskProperties: taskProperties,
	}
}

func (p PostgresCheckTask) Properties() workerpool.TaskProperties {
	return p.TaskProperties
}

func (p PostgresCheckTask) Run(ctx context.Context) error {

	p.logger.Info("Processing Job", zap.String("JobID", p.TaskProperties.ID.String()))

	p.logger.Info("Create configuration")
	p.logger.Info("Connecting to database")
	p.logger.Info("Get tables")
	p.logger.Info("Connecting to database")

	err := healthcheck.GetTables(ctx, p.logger)
	if err != nil {
		return err
	}

	return nil

}
