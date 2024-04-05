package compliance

import (
	"encoding/json"
	"fmt"

	"github.com/kaytu-io/kaytu-engine/pkg/auth/api"
	complianceApi "github.com/kaytu-io/kaytu-engine/pkg/compliance/api"
	"github.com/kaytu-io/kaytu-engine/pkg/compliance/runner"
	"github.com/kaytu-io/kaytu-engine/pkg/httpclient"
	onboardApi "github.com/kaytu-io/kaytu-engine/pkg/onboard/api"
	"go.uber.org/zap"
)

func (s *JobScheduler) runPublisher() error {
	s.logger.Info("runPublisher")
	ctx := &httpclient.Context{UserRole: api.InternalRole}

	connectionsMap := make(map[string]*onboardApi.Connection)
	connections, err := s.onboardClient.ListSources(ctx, nil)
	if err != nil {
		s.logger.Error("failed to get connections", zap.Error(err))
		return err
	}
	for _, connection := range connections {
		connection := connection
		connectionsMap[connection.ID.String()] = &connection
	}

	queries, err := s.complianceClient.ListQueries(ctx)
	if err != nil {
		s.logger.Error("failed to get queries", zap.Error(err))
		return err
	}
	queriesMap := make(map[string]*complianceApi.Query)
	for _, query := range queries {
		query := query
		queriesMap[query.ID] = &query
	}

	for i := 0; i < 10; i++ {
		err := s.db.UpdateTimeoutQueuedRunnerJobs()
		if err != nil {
			s.logger.Error("failed to update timed out runners", zap.Error(err))
		}

		err = s.db.UpdateTimedOutInProgressRunners()
		if err != nil {
			s.logger.Error("failed to update timed out runners", zap.Error(err))
		}

		runners, err := s.db.FetchCreatedRunners()
		if err != nil {
			s.logger.Error("failed to fetch created runners", zap.Error(err))
			continue
		}

		if len(runners) == 0 {
			break
		}

		for _, it := range runners {
			query, ok := queriesMap[it.QueryID]
			if !ok || query == nil {
				s.logger.Error("query not found", zap.String("queryId", it.QueryID), zap.Uint("runnerId", it.ID))
				continue
			}

			callers, err := it.GetCallers()
			if err != nil {
				s.logger.Error("failed to get callers", zap.Error(err), zap.Uint("runnerId", it.ID))
				continue
			}
			var providerConnectionID *string
			if it.ConnectionID != nil && *it.ConnectionID != "" {
				providerConnectionID = &connectionsMap[*it.ConnectionID].ConnectionID
			}
			job := runner.Job{
				ID:          it.ID,
				RetryCount:  it.RetryCount,
				ParentJobID: it.ParentJobID,
				CreatedAt:   it.CreatedAt,
				ExecutionPlan: runner.ExecutionPlan{
					Callers:              callers,
					Query:                *query,
					ConnectionID:         it.ConnectionID,
					ProviderConnectionID: providerConnectionID,
				},
			}

			jobJson, err := json.Marshal(job)
			if err != nil {
				_ = s.db.UpdateRunnerJob(job.ID, runner.ComplianceRunnerFailed, job.CreatedAt, nil, err.Error())
				s.logger.Error("failed to marshal job", zap.Error(err), zap.Uint("runnerId", it.ID))
				continue
			}

			s.logger.Info("publishing runner", zap.Uint("jobId", job.ID))
			if err := s.jq.Produce(ctx.Request().Context(), runner.JobQueueTopic, jobJson, fmt.Sprintf("job-%d-%d", job.ID, it.RetryCount)); err != nil {
				_ = s.db.UpdateRunnerJob(job.ID, runner.ComplianceRunnerFailed, job.CreatedAt, nil, err.Error())
				s.logger.Error("failed to send job", zap.Error(err), zap.Uint("runnerId", it.ID))
				continue
			}

			_ = s.db.UpdateRunnerJob(job.ID, runner.ComplianceRunnerQueued, job.CreatedAt, nil, "")
		}
	}

	err = s.db.RetryFailedRunners()
	if err != nil {
		s.logger.Error("failed to retry failed runners", zap.Error(err))
		return err
	}

	return nil
}
