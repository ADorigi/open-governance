package tasks

import (
	"context"

	"github.com/kaytu-io/open-governance/pkg/healthcheck"

	"github.com/adorigi/workerpool"
	"go.uber.org/zap"
)

type InternetCheckTask struct {
	workerpool.TaskProperties
	logger *zap.Logger
	url    string
}

func NewInternetCheckTask(logger *zap.Logger, taskProperties workerpool.TaskProperties, url string) *InternetCheckTask {
	return &InternetCheckTask{
		logger:         logger,
		TaskProperties: taskProperties,
		url:            url,
	}
}

func (p InternetCheckTask) Properties() workerpool.TaskProperties {
	return p.TaskProperties
}

func (p InternetCheckTask) Run(ctx context.Context) error {

	err := healthcheck.InternetURLCheck(p.url)
	if err != nil {
		return err
	}

	p.logger.Info("Internet reachable")
	return nil
}
