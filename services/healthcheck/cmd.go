package health

import (
	"context"

	"github.com/kaytu-io/open-governance/services/health/tasks"

	"github.com/adorigi/workerpool"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func Command() *cobra.Command {
	cmd := &cobra.Command{
		RunE: func(cmd *cobra.Command, _ []string) error {
			logger, err := zap.NewProduction()
			if err != nil {
				return err
			}

			pool := workerpool.NewWorkerPool(1)

			pool.Start(context.Background())

			postgrestask := tasks.NewPostgresCheckTask(
				logger,
				workerpool.TaskProperties{
					ID:          uuid.New(),
					Description: "Postgres check",
				},
			)

			pool.AddTask(postgrestask)

			internetask := tasks.NewInternetCheckTask(
				logger,
				workerpool.TaskProperties{
					ID:          uuid.New(),
					Description: "Internet check",
				},
				"https://google.com",
			)

			pool.AddTask(internetask)

			pool.Wait()

			return nil
		},
	}
	return cmd
}
