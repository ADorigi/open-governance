package query_runner

import (
	"context"
	authApi "github.com/kaytu-io/kaytu-util/pkg/api"
	"github.com/kaytu-io/kaytu-util/pkg/es"
	"github.com/kaytu-io/kaytu-util/pkg/httpclient"
	"github.com/kaytu-io/open-governance/pkg/inventory/api"
	"github.com/kaytu-io/open-governance/pkg/types"
	"go.uber.org/zap"
	"strconv"
	"time"
)

type Job struct {
	RunId       uint                 `json:"runID"`
	RetryCount  int                  `json:"retryCount"`
	CreatedBy   string               `json:"createdBy"`
	TriggeredAt int64                `json:"triggeredAt"`
	QueryId     string               `json:"queryId"`
	Parameters  []api.QueryParameter `json:"parameters"`
	Query       string               `json:"query"`
}

func (w *Worker) RunJob(ctx context.Context, job Job) error {
	queryResult, err := w.RunSQLNamedQuery(ctx, job.Query)
	if err != nil {
		return err
	}

	queryRunResult := types.QueryRunResult{
		RunId:       strconv.Itoa(int(job.RunId)),
		CreatedBy:   job.CreatedBy,
		TriggeredAt: job.TriggeredAt,
		EvaluatedAt: time.Now().UnixMilli(),
		QueryID:     job.QueryId,
		Parameters:  job.Parameters,
		ColumnNames: queryResult.Headers,
		Result:      queryResult.Result,
	}
	keys, idx := queryRunResult.KeysAndIndex()
	queryRunResult.EsID = es.HashOf(keys...)
	queryRunResult.EsIndex = idx

	var doc []es.Doc
	doc = append(doc, queryRunResult)

	if _, err := w.sinkClient.Ingest(&httpclient.Context{Ctx: ctx, UserRole: authApi.InternalRole}, doc); err != nil {
		w.logger.Error("Failed to sink Query Run Result", zap.String("RunID", strconv.Itoa(int(job.RunId))), zap.String("QueryID", job.QueryId), zap.Error(err))
		return err
	}

	return nil
}
