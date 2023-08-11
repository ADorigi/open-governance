package inventory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/kaytu-io/kaytu-engine/pkg/analytics/es/spend"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/kaytu-io/kaytu-engine/pkg/analytics/es/resource"
	es3 "github.com/kaytu-io/kaytu-engine/pkg/summarizer/es"
	"github.com/kaytu-io/kaytu-util/pkg/kafka"
	"github.com/kaytu-io/kaytu-util/pkg/keibi-es-sdk"

	awsSteampipe "github.com/kaytu-io/kaytu-aws-describer/pkg/steampipe"
	azureSteampipe "github.com/kaytu-io/kaytu-azure-describer/pkg/steampipe"
	analyticsDB "github.com/kaytu-io/kaytu-engine/pkg/analytics/db"
	authApi "github.com/kaytu-io/kaytu-engine/pkg/auth/api"
	insight "github.com/kaytu-io/kaytu-engine/pkg/insight/es"
	"github.com/kaytu-io/kaytu-engine/pkg/internal/httpclient"
	"github.com/kaytu-io/kaytu-engine/pkg/internal/httpserver"
	inventoryApi "github.com/kaytu-io/kaytu-engine/pkg/inventory/api"
	"github.com/kaytu-io/kaytu-engine/pkg/inventory/es"
	"github.com/kaytu-io/kaytu-engine/pkg/inventory/internal"
	"github.com/kaytu-io/kaytu-engine/pkg/utils"
	"github.com/kaytu-io/kaytu-util/pkg/model"
	"github.com/kaytu-io/kaytu-util/pkg/source"
	"github.com/kaytu-io/kaytu-util/pkg/steampipe"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const (
	EsFetchPageSize = 10000
	MaxConns        = 100
)

const (
	ConnectionIdParam    = "connectionId"
	ConnectionGroupParam = "connectionGroup"
)

func (h *HttpHandler) Register(e *echo.Echo) {
	v1 := e.Group("/api/v1")

	queryV1 := v1.Group("/query")
	queryV1.GET("", httpserver.AuthorizeHandler(h.ListQueries, authApi.ViewerRole))
	queryV1.POST("/run", httpserver.AuthorizeHandler(h.RunQuery, authApi.ViewerRole))
	queryV1.GET("/run/history", httpserver.AuthorizeHandler(h.GetRecentRanQueries, authApi.ViewerRole))

	v2 := e.Group("/api/v2")

	resourcesV2 := v2.Group("/resources")
	resourcesV2.GET("/count", httpserver.AuthorizeHandler(h.CountResources, authApi.ViewerRole))
	resourcesV2.GET("/metric/:resourceType", httpserver.AuthorizeHandler(h.GetResourceTypeMetricsHandler, authApi.ViewerRole))

	analyticsV2 := v2.Group("/analytics")
	analyticsV2.GET("/metric", httpserver.AuthorizeHandler(h.ListAnalyticsMetricsHandler, authApi.ViewerRole))
	analyticsV2.GET("/tag", httpserver.AuthorizeHandler(h.ListAnalyticsTags, authApi.ViewerRole))
	analyticsV2.GET("/trend", httpserver.AuthorizeHandler(h.ListAnalyticsMetricTrend, authApi.ViewerRole))
	analyticsV2.GET("/composition/:key", httpserver.AuthorizeHandler(h.ListAnalyticsComposition, authApi.ViewerRole))
	analyticsV2.GET("/regions/summary", httpserver.AuthorizeHandler(h.ListAnalyticsRegionsSummary, authApi.ViewerRole))
	analyticsV2.GET("/categories", httpserver.AuthorizeHandler(h.ListAnalyticsCategories, authApi.ViewerRole))

	analyticsSpend := analyticsV2.Group("/spend")
	analyticsSpend.GET("/metric", httpserver.AuthorizeHandler(h.ListAnalyticsSpendMetricsHandler, authApi.ViewerRole))
	analyticsSpend.GET("/composition", httpserver.AuthorizeHandler(h.ListAnalyticsSpendComposition, authApi.ViewerRole))
	analyticsSpend.GET("/trend", httpserver.AuthorizeHandler(h.GetAnalyticsSpendTrend, authApi.ViewerRole))
	analyticsSpend.GET("/metrics/trend", httpserver.AuthorizeHandler(h.GetAnalyticsSpendMetricsTrend, authApi.ViewerRole))
	analyticsSpend.GET("/table", httpserver.AuthorizeHandler(h.GetSpendTable, authApi.ViewerRole))

	connectionsV2 := v2.Group("/connections")
	connectionsV2.GET("/data", httpserver.AuthorizeHandler(h.ListConnectionsData, authApi.ViewerRole))
	connectionsV2.GET("/data/:connectionId", httpserver.AuthorizeHandler(h.GetConnectionData, authApi.ViewerRole))

	insightsV2 := v2.Group("/insights")
	insightsV2.GET("", httpserver.AuthorizeHandler(h.ListInsightResults, authApi.ViewerRole))
	insightsV2.GET("/:insightId/trend", httpserver.AuthorizeHandler(h.GetInsightTrendResults, authApi.ViewerRole))
	insightsV2.GET("/:insightId", httpserver.AuthorizeHandler(h.GetInsightResult, authApi.ViewerRole))

	metadata := v2.Group("/metadata")
	metadata.GET("/resourcetype", httpserver.AuthorizeHandler(h.ListResourceTypeMetadata, authApi.ViewerRole))

	v1.GET("/migrate-analytics", httpserver.AuthorizeHandler(h.MigrateAnalytics, authApi.AdminRole))
	v1.GET("/migrate-spend", httpserver.AuthorizeHandler(h.MigrateSpend, authApi.AdminRole))
}

func (h *HttpHandler) getConnectionIdFilterFromParams(ctx echo.Context) ([]string, error) {
	connectionIds := httpserver.QueryArrayParam(ctx, ConnectionIdParam)
	connectionGroup := ctx.QueryParam(ConnectionGroupParam)
	if len(connectionIds) == 0 && connectionGroup == "" {
		return nil, nil
	}

	if len(connectionIds) > 0 && connectionGroup != "" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "connectionId and connectionGroup cannot be used together")
	}

	if len(connectionIds) > 0 {
		return connectionIds, nil
	}

	connectionGroupObj, err := h.onboardClient.GetConnectionGroup(&httpclient.Context{UserRole: authApi.KeibiAdminRole}, connectionGroup)
	if err != nil {
		return nil, err
	}

	if len(connectionGroupObj.ConnectionIds) == 0 {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "connectionGroup has no connections")
	}

	return connectionGroupObj.ConnectionIds, nil
}

func (h *HttpHandler) MigrateAnalytics(ctx echo.Context) error {
	for i := 0; i < 1000; i++ {
		err := h.MigrateAnalyticsPart(i)
		if err != nil {
			return err
		}
	}
	return nil
}

func (h *HttpHandler) MigrateAnalyticsPart(summarizerJobID int) error {
	aDB := analyticsDB.NewDatabase(h.db.orm)

	connectionMap := map[string]resource.ConnectionMetricTrendSummary{}
	connectorMap := map[string]resource.ConnectorMetricTrendSummary{}

	resourceTypeMetricIDCache := map[string]string{}

	cctx := context.Background()

	pagination, err := es.NewConnectionResourceTypePaginator(
		h.client,
		[]keibi.BoolFilter{
			keibi.NewTermFilter("report_type", string(es3.ResourceTypeTrendConnectionSummary)),
			keibi.NewTermFilter("summarize_job_id", fmt.Sprintf("%d", summarizerJobID)),
		},
		nil,
	)
	if err != nil {
		return err
	}

	var docs []kafka.Doc
	for {
		if !pagination.HasNext() {
			fmt.Println("MigrateAnalytics = page done", summarizerJobID)
			break
		}

		fmt.Println("MigrateAnalytics = ask page", summarizerJobID)
		page, err := pagination.NextPage(cctx)
		if err != nil {
			return err
		}
		fmt.Println("MigrateAnalytics = next page", summarizerJobID)

		for _, hit := range page {
			connectionID, err := uuid.Parse(hit.SourceID)
			if err != nil {
				return err
			}

			var metricID string

			if v, ok := resourceTypeMetricIDCache[hit.ResourceType]; ok {
				metricID = v
			} else {
				metric, err := aDB.GetMetric(analyticsDB.MetricTypeAssets, hit.ResourceType)
				if err != nil {
					return err
				}

				if metric == nil {
					return fmt.Errorf("resource type %s not found", hit.ResourceType)
				}

				resourceTypeMetricIDCache[hit.ResourceType] = metric.ID
				metricID = metric.ID
			}

			if metricID == "" {
				continue
			}

			connection := resource.ConnectionMetricTrendSummary{
				ConnectionID:  connectionID,
				Connector:     hit.SourceType,
				EvaluatedAt:   hit.DescribedAt,
				MetricID:      metricID,
				ResourceCount: hit.ResourceCount,
			}
			key := fmt.Sprintf("%s-%s-%d", connectionID.String(), metricID, hit.SummarizeJobID)
			if v, ok := connectionMap[key]; ok {
				v.ResourceCount += connection.ResourceCount
				connectionMap[key] = v
			} else {
				connectionMap[key] = connection
			}

			connector := resource.ConnectorMetricTrendSummary{
				Connector:     hit.SourceType,
				EvaluatedAt:   hit.DescribedAt,
				MetricID:      metricID,
				ResourceCount: hit.ResourceCount,
			}
			key = fmt.Sprintf("%s-%s-%d", connector.Connector, metricID, hit.SummarizeJobID)
			if v, ok := connectorMap[key]; ok {
				v.ResourceCount += connector.ResourceCount
				connectorMap[key] = v
			} else {
				connectorMap[key] = connector
			}
		}
	}

	for _, c := range connectionMap {
		docs = append(docs, c)
	}

	for _, c := range connectorMap {
		docs = append(docs, c)
	}

	err = kafka.DoSend(h.kafkaProducer, "cloud-resources", -1, docs, h.logger)
	if err != nil {
		return err
	}
	return nil
}

func (h *HttpHandler) MigrateSpend(ctx echo.Context) error {
	for i := 0; i < 1000; i++ {
		err := h.MigrateSpendPart(i, true)
		if err != nil {
			return err
		}

		err = h.MigrateSpendPart(i, false)
		if err != nil {
			return err
		}
	}
	return nil
}

type ExistFilter struct {
	field string
}

func NewExistFilter(field string) keibi.BoolFilter {
	return ExistFilter{
		field: field,
	}
}
func (t ExistFilter) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"exists": map[string]string{
			"field": t.field,
		},
	})
}
func (t ExistFilter) IsBoolFilter() {}

func (h *HttpHandler) MigrateSpendPart(summarizerJobID int, isAWS bool) error {
	aDB := analyticsDB.NewDatabase(h.db.orm)
	connectionMap := map[string]spend.ConnectionMetricTrendSummary{}
	connectorMap := map[string]spend.ConnectorMetricTrendSummary{}

	cctx := context.Background()
	var boolFilters []keibi.BoolFilter
	if isAWS {
		boolFilters = []keibi.BoolFilter{
			keibi.NewTermFilter("report_type", string(es3.CostServiceSummaryDaily)),
			keibi.NewTermFilter("summarize_job_id", fmt.Sprintf("%d", summarizerJobID)),
			keibi.NewTermFilter("source_type", "AWS"),
			NewExistFilter("cost.Dimension1"),
		}
	} else {
		boolFilters = []keibi.BoolFilter{
			keibi.NewTermFilter("report_type", string(es3.CostServiceSummaryDaily)),
			keibi.NewTermFilter("summarize_job_id", fmt.Sprintf("%d", summarizerJobID)),
			keibi.NewTermFilter("source_type", "Azure"),
			NewExistFilter("cost.ServiceName"),
		}
	}

	pagination, err := es.NewConnectionCostPaginator(
		h.client,
		boolFilters,
		nil,
	)
	if err != nil {
		return err
	}
	serviceNameMetricCache := map[string]analyticsDB.AnalyticMetric{}

	var docs []kafka.Doc
	for {
		if !pagination.HasNext() {
			fmt.Println("MigrateAnalytics = page done", summarizerJobID)
			break
		}

		fmt.Println("MigrateAnalytics = ask page", summarizerJobID)
		page, err := pagination.NextPage(cctx)
		if err != nil {
			return err
		}
		fmt.Println("MigrateAnalytics = next page", summarizerJobID, len(page))

		for _, hit := range page {
			connectionID, err := uuid.Parse(hit.SourceID)
			if err != nil {
				return err
			}

			var metricID, metricName string
			if v, ok := serviceNameMetricCache[hit.ServiceName]; ok {
				metricID = v.ID
				metricName = v.Name
			} else {
				metric, err := aDB.GetMetric(analyticsDB.MetricTypeSpend, hit.ServiceName)
				if err != nil {
					return err
				}
				if metric == nil {
					return fmt.Errorf("GetMetric, table %s not found", hit.ServiceName)
				}
				serviceNameMetricCache[hit.ServiceName] = *metric
				metricID = metric.ID
				metricName = metric.Name
			}

			if metricID == "" {
				fmt.Println(hit.ServiceName, "doesnt have metricID")
				continue
			}

			conn, err := h.onboardClient.GetSource(&httpclient.Context{UserRole: authApi.AdminRole}, hit.SourceID)
			if err != nil {
				fmt.Println(err)
				continue
				//return err
			}

			dateTimestamp := (hit.PeriodStart + hit.PeriodEnd) / 2
			dateStr := time.Unix(dateTimestamp, 0).Format("2006-01-02")
			connection := spend.ConnectionMetricTrendSummary{
				ConnectionID:   connectionID,
				ConnectionName: conn.ConnectionName,
				Connector:      hit.Connector,
				Date:           dateStr,
				MetricID:       metricID,
				MetricName:     metricName,
				CostValue:      hit.CostValue,
				PeriodStart:    hit.PeriodStart * 1000,
				PeriodEnd:      hit.PeriodEnd * 1000,
			}
			key := fmt.Sprintf("%s-%s-%s", connectionID.String(), metricID, dateStr)
			if v, ok := connectionMap[key]; ok {
				v.CostValue += connection.CostValue
				connectionMap[key] = v
			} else {
				connectionMap[key] = connection
			}

			connector := spend.ConnectorMetricTrendSummary{
				Connector:   hit.Connector,
				Date:        dateStr,
				MetricID:    metricID,
				MetricName:  metricName,
				CostValue:   hit.CostValue,
				PeriodStart: hit.PeriodStart * 1000,
				PeriodEnd:   hit.PeriodEnd * 1000,
			}
			key = fmt.Sprintf("%s-%s-%s", connector.Connector, metricID, dateStr)
			if v, ok := connectorMap[key]; ok {
				v.CostValue += connector.CostValue
				connectorMap[key] = v
			} else {
				connectorMap[key] = connector
			}
		}
	}

	for _, c := range connectionMap {
		docs = append(docs, c)
	}

	for _, c := range connectorMap {
		docs = append(docs, c)
	}

	err = kafka.DoSend(h.kafkaProducer, "cloud-resources", -1, docs, h.logger)
	if err != nil {
		return err
	}
	return nil
}

func bindValidate(ctx echo.Context, i interface{}) error {
	if err := ctx.Bind(i); err != nil {
		return err
	}

	if err := ctx.Validate(i); err != nil {
		return err
	}

	return nil
}

func (h *HttpHandler) getConnectorTypesFromConnectionIDs(ctx echo.Context, connectorTypes []source.Type, connectionIDs []string) ([]source.Type, error) {
	if len(connectionIDs) == 0 {
		return connectorTypes, nil
	}
	if len(connectorTypes) != 0 {
		return connectorTypes, nil
	}
	connections, err := h.onboardClient.GetSources(httpclient.FromEchoContext(ctx), connectionIDs)
	if err != nil {
		return nil, err
	}

	enabledConnectors := make(map[source.Type]bool)
	for _, connection := range connections {
		enabledConnectors[connection.Connector] = true
	}
	connectorTypes = make([]source.Type, 0, len(enabledConnectors))
	for connectorType := range enabledConnectors {
		connectorTypes = append(connectorTypes, connectorType)
	}

	return connectorTypes, nil
}

func (h *HttpHandler) ListAnalyticsMetrics(metricIDs []string, metricType analyticsDB.MetricType, tagMap map[string][]string, connectorTypes []source.Type, connectionIDs []string, minCount int, timeAt time.Time) (int, []inventoryApi.Metric, error) {
	aDB := analyticsDB.NewDatabase(h.db.orm)

	mts, err := aDB.ListFilteredMetrics(tagMap, metricType, metricIDs, connectorTypes)
	if err != nil {
		return 0, nil, err
	}
	filteredMetricIDs := make([]string, 0, len(mts))
	for _, metric := range mts {
		filteredMetricIDs = append(filteredMetricIDs, metric.ID)
	}

	var metricIndexed map[string]int
	if len(connectionIDs) > 0 {
		metricIndexed, err = es.FetchConnectionAnalyticMetricCountAtTime(h.client, connectorTypes, connectionIDs, timeAt, filteredMetricIDs, EsFetchPageSize)
	} else {
		metricIndexed, err = es.FetchConnectorAnalyticMetricCountAtTime(h.client, connectorTypes, timeAt, filteredMetricIDs, EsFetchPageSize)
	}
	if err != nil {
		return 0, nil, err
	}

	apiMetrics := make([]inventoryApi.Metric, 0, len(mts))
	totalCount := 0
	for _, metric := range mts {
		apiMetric := inventoryApi.MetricToAPI(metric)
		if count, ok := metricIndexed[metric.ID]; ok && count >= minCount {
			apiMetric.Count = &count
			totalCount += count
		}
		if (minCount == 0) || (apiMetric.Count != nil && *apiMetric.Count >= minCount) {
			apiMetrics = append(apiMetrics, apiMetric)
		}
	}

	return totalCount, apiMetrics, nil
}

// ListAnalyticsMetricsHandler godoc
//
//	@Summary		List analytics metrics
//	@Description	Retrieving list of analytics with metrics of each type based on the given input filters.
//	@Security		BearerToken
//	@Tags			analytics
//	@Accept			json
//	@Produce		json
//	@Param			tag				query		[]string		false	"Key-Value tags in key=value format to filter by"
//	@Param			metricType		query		string			false	"Metric type, default: assets"	Enums(assets, spend)
//	@Param			connector		query		[]source.Type	false	"Connector type to filter by"
//	@Param			connectionId	query		[]string		false	"Connection IDs to filter by - mutually exclusive with connectionGroup"
//	@Param			connectionGroup	query		string			false	"Connection group to filter by - mutually exclusive with connectionId"
//	@Param			metricIDs		query		[]string		false	"Metric IDs"
//	@Param			endTime			query		string			false	"timestamp for resource count in epoch seconds"
//	@Param			startTime		query		string			false	"timestamp for resource count change comparison in epoch seconds"
//	@Param			minCount		query		int				false	"Minimum number of resources with this tag value, default 1"
//	@Param			sortBy			query		string			false	"Sort by field - default is count"	Enums(name,count,growth,growth_rate)
//	@Param			pageSize		query		int				false	"page size - default is 20"
//	@Param			pageNumber		query		int				false	"page number - default is 1"
//	@Success		200				{object}	inventoryApi.ListMetricsResponse
//	@Router			/inventory/api/v2/analytics/metric [get]
func (h *HttpHandler) ListAnalyticsMetricsHandler(ctx echo.Context) error {
	var err error
	tagMap := model.TagStringsToTagMap(httpserver.QueryArrayParam(ctx, "tag"))
	metricType := analyticsDB.MetricType(ctx.QueryParam("metricType"))
	if metricType == "" {
		metricType = analyticsDB.MetricTypeAssets
	}
	connectorTypes := source.ParseTypes(httpserver.QueryArrayParam(ctx, "connector"))
	connectionIDs, err := h.getConnectionIdFilterFromParams(ctx)
	if err != nil {
		return err
	}
	if len(connectionIDs) > MaxConns {
		return ctx.JSON(http.StatusBadRequest, "too many connections")
	}
	metricIDs := httpserver.QueryArrayParam(ctx, "metricIDs")

	connectorTypes, err = h.getConnectorTypesFromConnectionIDs(ctx, connectorTypes, connectionIDs)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, err.Error())
	}
	endTime := time.Now()
	if endTimeStr := ctx.QueryParam("endTime"); endTimeStr != "" {
		endTimeVal, err := strconv.ParseInt(endTimeStr, 10, 64)
		if err != nil {
			return ctx.JSON(http.StatusBadRequest, "invalid endTime value")
		}
		endTime = time.Unix(endTimeVal, 0)
	}
	startTime := endTime.AddDate(0, 0, -7)
	if startTimeStr := ctx.QueryParam("startTime"); startTimeStr != "" {
		startTimeVal, err := strconv.ParseInt(startTimeStr, 10, 64)
		if err != nil {
			return ctx.JSON(http.StatusBadRequest, "invalid startTime value")
		}
		startTime = time.Unix(startTimeVal, 0)
	}
	minCount := 1
	if minCountStr := ctx.QueryParam("minCount"); minCountStr != "" {
		minCountVal, err := strconv.ParseInt(minCountStr, 10, 64)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "minCount must be a number")
		}
		minCount = int(minCountVal)
	}
	pageNumber, pageSize, err := utils.PageConfigFromStrings(ctx.QueryParam("pageNumber"), ctx.QueryParam("pageSize"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, err.Error())
	}
	sortBy := strings.ToLower(ctx.QueryParam("sortBy"))
	if sortBy == "" {
		sortBy = "count"
	}
	if sortBy != "name" && sortBy != "count" &&
		sortBy != "growth" && sortBy != "growth_rate" {
		return ctx.JSON(http.StatusBadRequest, "invalid sortBy value")
	}

	totalCount, apiMetrics, err := h.ListAnalyticsMetrics(metricIDs, metricType, tagMap, connectorTypes, connectionIDs, minCount, endTime)
	if err != nil {
		return err
	}
	apiMetricsMap := make(map[string]inventoryApi.Metric, len(apiMetrics))
	for _, apiMetric := range apiMetrics {
		apiMetricsMap[apiMetric.ID] = apiMetric
	}

	totalOldCount, oldApiMetrics, err := h.ListAnalyticsMetrics(metricIDs, metricType, tagMap, connectorTypes, connectionIDs, 0, startTime)
	if err != nil {
		return err
	}
	for _, oldApiMetric := range oldApiMetrics {
		if apiMetric, ok := apiMetricsMap[oldApiMetric.ID]; ok {
			apiMetric.OldCount = oldApiMetric.Count
			apiMetricsMap[oldApiMetric.ID] = apiMetric
		}
	}

	apiMetrics = make([]inventoryApi.Metric, 0, len(apiMetricsMap))
	for _, apiMetric := range apiMetricsMap {
		apiMetrics = append(apiMetrics, apiMetric)
	}

	sort.Slice(apiMetrics, func(i, j int) bool {
		switch sortBy {
		case "name":
			return apiMetrics[i].Name < apiMetrics[j].Name
		case "count":
			if apiMetrics[i].Count == nil && apiMetrics[j].Count == nil {
				break
			}
			if apiMetrics[i].Count == nil {
				return false
			}
			if apiMetrics[j].Count == nil {
				return true
			}
			if *apiMetrics[i].Count != *apiMetrics[j].Count {
				return *apiMetrics[i].Count > *apiMetrics[j].Count
			}
		case "growth":
			diffi := utils.PSub(apiMetrics[i].Count, apiMetrics[i].OldCount)
			diffj := utils.PSub(apiMetrics[j].Count, apiMetrics[j].OldCount)
			if diffi == nil && diffj == nil {
				break
			}
			if diffi == nil {
				return false
			}
			if diffj == nil {
				return true
			}
			if *diffi != *diffj {
				return *diffi > *diffj
			}
		case "growth_rate":
			diffi := utils.PSub(apiMetrics[i].Count, apiMetrics[i].OldCount)
			diffj := utils.PSub(apiMetrics[j].Count, apiMetrics[j].OldCount)
			if diffi == nil && diffj == nil {
				break
			}
			if diffi == nil {
				return false
			}
			if diffj == nil {
				return true
			}
			if apiMetrics[i].OldCount == nil && apiMetrics[j].OldCount == nil {
				break
			}
			if apiMetrics[i].OldCount == nil {
				return true
			}
			if apiMetrics[j].OldCount == nil {
				return false
			}
			if *apiMetrics[i].OldCount == 0 && *apiMetrics[j].OldCount == 0 {
				break
			}
			if *apiMetrics[i].OldCount == 0 {
				return false
			}
			if *apiMetrics[j].OldCount == 0 {
				return true
			}
			if float64(*diffi)/float64(*apiMetrics[i].OldCount) != float64(*diffj)/float64(*apiMetrics[j].OldCount) {
				return float64(*diffi)/float64(*apiMetrics[i].OldCount) > float64(*diffj)/float64(*apiMetrics[j].OldCount)
			}
		}
		return apiMetrics[i].Name < apiMetrics[j].Name
	})

	result := inventoryApi.ListMetricsResponse{
		TotalCount:    totalCount,
		TotalOldCount: totalOldCount,
		TotalMetrics:  len(apiMetrics),
		Metrics:       utils.Paginate(pageNumber, pageSize, apiMetrics),
	}

	return ctx.JSON(http.StatusOK, result)
}

// ListAnalyticsTags godoc
//
//	@Summary		List analytics tags
//	@Description	Retrieving a list of tag keys with their possible values for all analytic metrics.
//	@Security		BearerToken
//	@Tags			analytics
//	@Accept			json
//	@Produce		json
//	@Param			connector		query		[]string	false	"Connector type to filter by"
//	@Param			connectionId	query		[]string	false	"Connection IDs to filter by - mutually exclusive with connectionGroup"
//	@Param			connectionGroup	query		string		false	"Connection group to filter by - mutually exclusive with connectionId"
//	@Param			minCount		query		int			false	"Minimum number of resources/spend with this tag value, default 1"
//	@Param			startTime		query		int			false	"Start time in unix timestamp format, default now - 1 month"
//	@Param			endTime			query		int			false	"End time in unix timestamp format, default now"
//	@Param			metricType		query		string		false	"Metric type, default: assets"	Enums(assets, spend)
//	@Success		200				{object}	map[string][]string
//	@Router			/inventory/api/v2/analytics/tag [get]
func (h *HttpHandler) ListAnalyticsTags(ctx echo.Context) error {
	connectorTypes := source.ParseTypes(httpserver.QueryArrayParam(ctx, "connector"))
	connectionIDs, err := h.getConnectionIdFilterFromParams(ctx)
	if len(connectionIDs) > MaxConns {
		return ctx.JSON(http.StatusBadRequest, "too many connections")
	}
	connectorTypes, err = h.getConnectorTypesFromConnectionIDs(ctx, connectorTypes, connectionIDs)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, err.Error())
	}
	minCount := 1
	if minCountStr := ctx.QueryParam("minCount"); minCountStr != "" {
		minCountVal, err := strconv.ParseInt(minCountStr, 10, 64)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "minCount must be a number")
		}
		minCount = int(minCountVal)
	}
	minAmount := float64(minCount)
	endTime, err := utils.TimeFromQueryParam(ctx, "endTime", time.Now())
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "endTime must be a number")
	}
	startTime, err := utils.TimeFromQueryParam(ctx, "startTime", endTime.AddDate(0, -1, 0))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "startTime must be a number")
	}

	metricType := analyticsDB.MetricType(ctx.QueryParam("metricType"))
	if metricType == "" {
		metricType = analyticsDB.MetricTypeAssets
	}

	aDB := analyticsDB.NewDatabase(h.db.orm)
	fmt.Println("connectorTypes", connectorTypes)
	tags, err := aDB.ListMetricTagsKeysWithPossibleValues(connectorTypes)
	if err != nil {
		return err
	}
	tags = model.TrimPrivateTags(tags)

	var metricCount map[string]int
	var spend map[string]float64

	if metricType == analyticsDB.MetricTypeAssets {
		if len(connectionIDs) > 0 {
			metricCount, err = es.FetchConnectionAnalyticMetricCountAtTime(h.client, connectorTypes, connectionIDs, endTime, nil, EsFetchPageSize)
		} else {
			metricCount, err = es.FetchConnectorAnalyticMetricCountAtTime(h.client, connectorTypes, endTime, nil, EsFetchPageSize)
		}
		if err != nil {
			return err
		}
	} else {
		spend, err = es.FetchSpendByMetric(h.client, connectionIDs, connectorTypes, nil, startTime, endTime, EsFetchPageSize)
		if err != nil {
			return err
		}
	}

	fmt.Println("metricCount", metricCount)
	fmt.Println("spend", spend)
	fmt.Println("tags", tags)

	filteredTags := map[string][]string{}
	for key, values := range tags {
		for _, tagValue := range values {
			metrics, err := aDB.ListFilteredMetrics(map[string][]string{
				key: {tagValue},
			}, metricType, nil, connectorTypes)
			if err != nil {
				return err
			}

			fmt.Println("metrics", key, tagValue, metrics)
			for _, metric := range metrics {
				if (metric.Type == analyticsDB.MetricTypeAssets && metricCount[metric.ID] >= minCount) ||
					(metric.Type == analyticsDB.MetricTypeSpend && spend[metric.ID] >= minAmount) {
					filteredTags[key] = append(filteredTags[key], tagValue)
					break
				}
			}
		}
	}
	tags = filteredTags
	fmt.Println("filteredTags", filteredTags)

	return ctx.JSON(http.StatusOK, tags)
}

// ListAnalyticsMetricTrend godoc
//
//	@Summary		Get metric trend
//
//	@Description	Retrieving a list of resource counts over the course of the specified time frame based on the given input filters
//	@Security		BearerToken
//	@Tags			analytics
//	@Accept			json
//	@Produce		json
//	@Param			tag				query		[]string		false	"Key-Value tags in key=value format to filter by"
//	@Param			metricType		query		string			false	"Metric type, default: assets"	Enums(assets, spend)
//	@Param			ids				query		[]string		false	"Metric IDs to filter by"
//	@Param			connector		query		[]source.Type	false	"Connector type to filter by"
//	@Param			connectionId	query		[]string		false	"Connection IDs to filter by - mutually exclusive with connectionGroup"
//	@Param			connectionGroup	query		string			false	"Connection group to filter by - mutually exclusive with connectionId"
//	@Param			startTime		query		string			false	"timestamp for start in epoch seconds"
//	@Param			endTime			query		string			false	"timestamp for end in epoch seconds"
//	@Param			datapointCount	query		string			false	"maximum number of datapoints to return, default is 30"
//	@Success		200				{object}	[]inventoryApi.ResourceTypeTrendDatapoint
//	@Router			/inventory/api/v2/analytics/trend [get]
func (h *HttpHandler) ListAnalyticsMetricTrend(ctx echo.Context) error {
	var err error
	aDB := analyticsDB.NewDatabase(h.db.orm)
	tagMap := model.TagStringsToTagMap(httpserver.QueryArrayParam(ctx, "tag"))
	metricType := analyticsDB.MetricType(ctx.QueryParam("metricType"))
	if metricType == "" {
		metricType = analyticsDB.MetricTypeAssets
	}
	ids := httpserver.QueryArrayParam(ctx, "ids")
	connectorTypes := source.ParseTypes(httpserver.QueryArrayParam(ctx, "connector"))
	connectionIDs, err := h.getConnectionIdFilterFromParams(ctx)
	if err != nil {
		return err
	}
	if len(connectionIDs) > MaxConns {
		return echo.NewHTTPError(http.StatusBadRequest, "too many connections")
	}

	endTimeStr := ctx.QueryParam("endTime")
	endTime := time.Now()
	if endTimeStr != "" {
		endTimeVal, err := strconv.ParseInt(endTimeStr, 10, 64)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid time")
		}
		endTime = time.Unix(endTimeVal, 0)
	}
	startTimeStr := ctx.QueryParam("startTime")
	startTime := endTime.AddDate(0, -1, 0)
	if startTimeStr != "" {
		startTimeVal, err := strconv.ParseInt(startTimeStr, 10, 64)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid time")
		}
		startTime = time.Unix(startTimeVal, 0)
	}

	datapointCountStr := ctx.QueryParam("datapointCount")
	datapointCount := int64(30)
	if datapointCountStr != "" {
		datapointCount, err = strconv.ParseInt(datapointCountStr, 10, 64)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid datapointCount")
		}
	}

	metrics, err := aDB.ListFilteredMetrics(tagMap, metricType, ids, connectorTypes)
	if err != nil {
		return err
	}
	metricIDs := make([]string, 0, len(metrics))
	for _, metric := range metrics {
		metricIDs = append(metricIDs, metric.ID)
	}

	timeToCountMap := make(map[int]int)
	if endTime.Round(24 * time.Hour).Before(endTime) {
		endTime = endTime.Round(24 * time.Hour).Add(24 * time.Hour)
	} else {
		endTime = endTime.Round(24 * time.Hour)
	}
	if startTime.Round(24 * time.Hour).After(startTime) {
		startTime = startTime.Round(24 * time.Hour).Add(-24 * time.Hour)
	} else {
		startTime = startTime.Round(24 * time.Hour)
	}

	esDatapointCount := int(math.Floor(endTime.Sub(startTime).Hours() / 24))
	if esDatapointCount == 0 {
		esDatapointCount = 1
	}
	if len(connectionIDs) != 0 {
		timeToCountMap, err = es.FetchConnectionMetricTrendSummaryPage(h.client, connectionIDs, metricIDs, startTime, endTime, esDatapointCount, EsFetchPageSize)
		if err != nil {
			return err
		}
	} else {
		timeToCountMap, err = es.FetchConnectorMetricTrendSummaryPage(h.client, connectorTypes, metricIDs, startTime, endTime, esDatapointCount, EsFetchPageSize)
		if err != nil {
			return err
		}
	}

	apiDatapoints := make([]inventoryApi.ResourceTypeTrendDatapoint, 0, len(timeToCountMap))
	for timeAt, count := range timeToCountMap {
		apiDatapoints = append(apiDatapoints, inventoryApi.ResourceTypeTrendDatapoint{Count: count, Date: time.UnixMilli(int64(timeAt))})
	}
	sort.Slice(apiDatapoints, func(i, j int) bool {
		return apiDatapoints[i].Date.Before(apiDatapoints[j].Date)
	})
	apiDatapoints = internal.DownSampleResourceTypeTrendDatapoints(apiDatapoints, int(datapointCount))

	return ctx.JSON(http.StatusOK, apiDatapoints)
}

// ListAnalyticsComposition godoc
//
//	@Summary		List analytics composition
//	@Description	Retrieving tag values with the most resources for the given key.
//	@Security		BearerToken
//	@Tags			analytics
//	@Accept			json
//	@Produce		json
//	@Param			key				path		string			true	"Tag key"
//	@Param			metricType		query		string			false	"Metric type, default: assets"	Enums(assets, spend)
//	@Param			top				query		int				true	"How many top values to return default is 5"
//	@Param			connector		query		[]source.Type	false	"Connector types to filter by"
//	@Param			connectionId	query		[]string		false	"Connection IDs to filter by - mutually exclusive with connectionGroup"
//	@Param			connectionGroup	query		string			false	"Connection group to filter by - mutually exclusive with connectionId"
//	@Param			endTime			query		string			false	"timestamp for resource count in epoch seconds"
//	@Param			startTime		query		string			false	"timestamp for resource count change comparison in epoch seconds"
//	@Success		200				{object}	inventoryApi.ListResourceTypeCompositionResponse
//	@Router			/inventory/api/v2/analytics/composition/{key} [get]
func (h *HttpHandler) ListAnalyticsComposition(ctx echo.Context) error {
	aDB := analyticsDB.NewDatabase(h.db.orm)

	var err error
	tagKey := ctx.Param("key")
	if tagKey == "" || strings.HasPrefix(tagKey, model.KaytuPrivateTagPrefix) {
		return echo.NewHTTPError(http.StatusBadRequest, "tag key is invalid")
	}
	metricType := analyticsDB.MetricType(ctx.QueryParam("metricType"))
	if metricType == "" {
		metricType = analyticsDB.MetricTypeAssets
	}

	topStr := ctx.QueryParam("top")
	top := int64(5)
	if topStr != "" {
		top, err = strconv.ParseInt(topStr, 10, 64)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid top value")
		}

	}
	connectorTypes := source.ParseTypes(httpserver.QueryArrayParam(ctx, "connector"))
	connectionIDs, err := h.getConnectionIdFilterFromParams(ctx)
	if err != nil {
		return err
	}
	if len(connectionIDs) > MaxConns {
		return ctx.JSON(http.StatusBadRequest, "too many connections")
	}

	endTime := time.Now()
	if endTimeStr := ctx.QueryParam("endTime"); endTimeStr != "" {
		endTimeVal, err := strconv.ParseInt(endTimeStr, 10, 64)
		if err != nil {
			return ctx.JSON(http.StatusBadRequest, "invalid endTime value")
		}
		endTime = time.Unix(endTimeVal, 0)
	}
	startTime := endTime.AddDate(0, 0, -7)
	if startTimeStr := ctx.QueryParam("startTime"); startTimeStr != "" {
		startTimeVal, err := strconv.ParseInt(startTimeStr, 10, 64)
		if err != nil {
			return ctx.JSON(http.StatusBadRequest, "invalid startTime value")
		}
		startTime = time.Unix(startTimeVal, 0)
	}

	filteredMetrics, err := aDB.ListFilteredMetrics(map[string][]string{tagKey: nil}, metricType, nil, connectorTypes)
	if err != nil {
		return err
	}
	var metrics []analyticsDB.AnalyticMetric
	for _, metric := range filteredMetrics {
		metrics = append(metrics, metric)
	}
	metricsIDs := make([]string, 0, len(metrics))
	for _, metric := range metrics {
		metricsIDs = append(metricsIDs, metric.ID)
	}

	var metricIndexed map[string]int
	if len(connectionIDs) > 0 {
		metricIndexed, err = es.FetchConnectionAnalyticMetricCountAtTime(h.client, connectorTypes, connectionIDs, endTime, metricsIDs, EsFetchPageSize)
	} else {
		metricIndexed, err = es.FetchConnectorAnalyticMetricCountAtTime(h.client, connectorTypes, endTime, metricsIDs, EsFetchPageSize)
	}
	if err != nil {
		return err
	}

	var oldMetricIndexed map[string]int
	if len(connectionIDs) > 0 {
		oldMetricIndexed, err = es.FetchConnectionAnalyticMetricCountAtTime(h.client, connectorTypes, connectionIDs, startTime, metricsIDs, EsFetchPageSize)
	} else {
		oldMetricIndexed, err = es.FetchConnectorAnalyticMetricCountAtTime(h.client, connectorTypes, startTime, metricsIDs, EsFetchPageSize)
	}
	if err != nil {
		return err
	}

	type currentAndOldCount struct {
		current int
		old     int
	}

	valueCountMap := make(map[string]currentAndOldCount)
	totalCount := 0
	totalOldCount := 0
	for _, metric := range metrics {
		for _, tagValue := range metric.GetTagsMap()[tagKey] {
			if _, ok := valueCountMap[tagValue]; !ok {
				valueCountMap[tagValue] = currentAndOldCount{}
			}
			v := valueCountMap[tagValue]
			v.current += metricIndexed[metric.ID]
			v.old += oldMetricIndexed[metric.ID]
			totalCount += metricIndexed[metric.ID]
			totalOldCount += oldMetricIndexed[metric.ID]
			valueCountMap[tagValue] = v
			break
		}
	}

	type strIntPair struct {
		str    string
		counts currentAndOldCount
	}
	valueCountPairs := make([]strIntPair, 0, len(valueCountMap))
	for value, count := range valueCountMap {
		valueCountPairs = append(valueCountPairs, strIntPair{str: value, counts: count})
	}
	sort.Slice(valueCountPairs, func(i, j int) bool {
		return valueCountPairs[i].counts.current > valueCountPairs[j].counts.current
	})

	apiResult := inventoryApi.ListResourceTypeCompositionResponse{
		TotalCount:      totalCount,
		TotalValueCount: len(valueCountMap),
		TopValues:       make(map[string]inventoryApi.CountPair),
		Others:          inventoryApi.CountPair{},
	}

	for i, pair := range valueCountPairs {
		if i < int(top) {
			apiResult.TopValues[pair.str] = inventoryApi.CountPair{
				Count:    pair.counts.current,
				OldCount: pair.counts.old,
			}
		} else {
			apiResult.Others.Count += pair.counts.current
			apiResult.Others.OldCount += pair.counts.old
		}
	}

	return ctx.JSON(http.StatusOK, apiResult)
}

// ListAnalyticsRegionsSummary godoc
//
//	@Summary		List Regions Summary
//	@Description	Retrieving list of regions analytics summary
//	@Security		BearerToken
//	@Tags			analytics
//	@Accept			json
//	@Produce		json
//	@Param			connector		query		[]source.Type	false	"Connector type to filter by"
//	@Param			connectionId	query		[]string		false	"Connection IDs to filter by - mutually exclusive with connectionGroup"
//	@Param			connectionGroup	query		string			false	"Connection group to filter by - mutually exclusive with connectionId"
//	@Param			startTime		query		int				false	"start time in unix seconds - default is now"
//	@Param			endTime			query		int				false	"end time in unix seconds - default is one week ago"
//	@Param			sortBy			query		string			false	"column to sort by - default is resource_count"	Enums(resource_count, growth, growth_rate)
//	@Param			pageSize		query		int				false	"page size - default is 20"
//	@Param			pageNumber		query		int				false	"page number - default is 1"
//	@Success		200				{object}	inventoryApi.RegionsResourceCountResponse
//	@Router			/inventory/api/v2/analytics/regions/summary [get]
func (h *HttpHandler) ListAnalyticsRegionsSummary(ctx echo.Context) error {
	connectors := source.ParseTypes(httpserver.QueryArrayParam(ctx, "connector"))
	endTimeStr := ctx.QueryParam("endTime")
	endTime := time.Now()
	if endTimeStr != "" {
		endTimeUnix, err := strconv.ParseInt(endTimeStr, 10, 64)
		if err != nil {
			return ctx.JSON(http.StatusBadRequest, "endTime is not a valid integer")
		}
		endTime = time.Unix(endTimeUnix, 0)
	}
	startTimeStr := ctx.QueryParam("startTime")
	startTime := endTime.AddDate(0, 0, -7)
	if startTimeStr != "" {
		startTimeUnix, err := strconv.ParseInt(startTimeStr, 10, 64)
		if err != nil {
			return ctx.JSON(http.StatusBadRequest, "startTime is not a valid integer")
		}
		startTime = time.Unix(startTimeUnix, 0)
	}
	connectionIDs, err := h.getConnectionIdFilterFromParams(ctx)
	if err != nil {
		return err
	}

	pageNumber, pageSize, err := utils.PageConfigFromStrings(ctx.QueryParam("pageNumber"), ctx.QueryParam("pageSize"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, err.Error())
	}
	sortBy := ctx.QueryParam("sortBy")
	if sortBy == "" {
		sortBy = "resource_count"
	}

	currentLocationDistribution, err := es.FetchRegionSummaryPage(h.client, connectors, connectionIDs, nil, endTime, 10000)
	if err != nil {
		return err
	}

	oldLocationDistribution, err := es.FetchRegionSummaryPage(h.client, connectors, connectionIDs, nil, startTime, 10000)
	if err != nil {
		return err
	}

	var locationResponses []inventoryApi.LocationResponse
	for region, count := range currentLocationDistribution {
		cnt := count
		oldCount := 0
		if value, ok := oldLocationDistribution[region]; ok {
			oldCount = value
		}
		locationResponses = append(locationResponses, inventoryApi.LocationResponse{
			Location:         region,
			ResourceCount:    &cnt,
			ResourceOldCount: &oldCount,
		})
	}

	sort.Slice(locationResponses, func(i, j int) bool {
		switch sortBy {
		case "resource_count":
			if locationResponses[i].ResourceCount == nil && locationResponses[j].ResourceCount == nil {
				break
			}
			if locationResponses[i].ResourceCount == nil {
				return false
			}
			if locationResponses[j].ResourceCount == nil {
				return true
			}
			if *locationResponses[i].ResourceCount != *locationResponses[j].ResourceCount {
				return *locationResponses[i].ResourceCount > *locationResponses[j].ResourceCount
			}
		case "growth":
			diffi := utils.PSub(locationResponses[i].ResourceCount, locationResponses[i].ResourceOldCount)
			diffj := utils.PSub(locationResponses[j].ResourceCount, locationResponses[j].ResourceOldCount)
			if diffi == nil && diffj == nil {
				break
			}
			if diffi == nil {
				return false
			}
			if diffj == nil {
				return true
			}
			if *diffi != *diffj {
				return *diffi > *diffj
			}
		case "growth_rate":
			diffi := utils.PSub(locationResponses[i].ResourceCount, locationResponses[i].ResourceOldCount)
			diffj := utils.PSub(locationResponses[j].ResourceCount, locationResponses[j].ResourceOldCount)
			if diffi == nil && diffj == nil {
				break
			}
			if diffi == nil {
				return false
			}
			if diffj == nil {
				return true
			}
			if locationResponses[i].ResourceOldCount == nil && locationResponses[j].ResourceOldCount == nil {
				break
			}
			if locationResponses[i].ResourceOldCount == nil {
				return true
			}
			if locationResponses[j].ResourceOldCount == nil {
				return false
			}
			if *locationResponses[i].ResourceOldCount == 0 && *locationResponses[j].ResourceOldCount == 0 {
				break
			}
			if *locationResponses[i].ResourceOldCount == 0 {
				return false
			}
			if *locationResponses[j].ResourceOldCount == 0 {
				return true
			}
			if float64(*diffi)/float64(*locationResponses[i].ResourceOldCount) != float64(*diffj)/float64(*locationResponses[j].ResourceOldCount) {
				return float64(*diffi)/float64(*locationResponses[i].ResourceOldCount) > float64(*diffj)/float64(*locationResponses[j].ResourceOldCount)
			}
		}
		return locationResponses[i].Location < locationResponses[j].Location
	})

	response := inventoryApi.RegionsResourceCountResponse{
		TotalCount: len(locationResponses),
		Regions:    utils.Paginate(pageNumber, pageSize, locationResponses),
	}

	return ctx.JSON(http.StatusOK, response)
}

// ListAnalyticsCategories godoc
//
//	@Summary		List Analytics categories
//	@Description	Retrieving list of categories for analytics
//	@Security		BearerToken
//	@Tags			analytics
//	@Accept			json
//	@Produce		json
//	@Param			metricType	query		string	false	"Metric type, default: assets"	Enums(assets, spend)
//	@Success		200			{object}	inventoryApi.AnalyticsCategoriesResponse
//	@Router			/inventory/api/v2/analytics/categories [get]
func (h *HttpHandler) ListAnalyticsCategories(ctx echo.Context) error {
	aDB := analyticsDB.NewDatabase(h.db.orm)

	metricType := analyticsDB.MetricType(ctx.QueryParam("metricType"))
	if metricType == "" {
		metricType = analyticsDB.MetricTypeAssets
	}

	metrics, err := aDB.ListMetrics()
	if err != nil {
		return err
	}

	categoryResourceTypeMap := map[string][]string{}
	for _, metric := range metrics {
		if metric.Type != metricType {
			continue
		}

		for _, tag := range metric.Tags {
			if tag.Key == "category" {
				for _, category := range tag.GetValue() {
					categoryResourceTypeMap[category] = append(
						categoryResourceTypeMap[category],
						metric.Tables...,
					)
				}
			}
		}
	}

	return ctx.JSON(http.StatusOK, inventoryApi.AnalyticsCategoriesResponse{
		CategoryResourceType: categoryResourceTypeMap,
	})
}

// ListAnalyticsSpendMetricsHandler godoc
//
//	@Summary		List spend metrics
//	@Description	Retrieving cost metrics with respect to specified filters. The API returns information such as the total cost and costs per each service based on the specified filters.
//	@Security		BearerToken
//	@Tags			analytics
//	@Accept			json
//	@Produce		json
//	@Param			connector		query		[]source.Type	false	"Connector type to filter by"
//	@Param			connectionId	query		[]string		false	"Connection IDs to filter by - mutually exclusive with connectionGroup"
//	@Param			connectionGroup	query		string			false	"Connection group to filter by - mutually exclusive with connectionId"
//	@Param			startTime		query		string			false	"timestamp for start in epoch seconds"
//	@Param			endTime			query		string			false	"timestamp for end in epoch seconds"
//	@Param			sortBy			query		string			false	"Sort by field - default is cost"	Enums(dimension,cost,growth,growth_rate)
//	@Param			pageSize		query		int				false	"page size - default is 20"
//	@Param			pageNumber		query		int				false	"page number - default is 1"
//	@Success		200				{object}	inventoryApi.ListCostMetricsResponse
//	@Router			/inventory/api/v2/analytics/spend/metric [get]
func (h *HttpHandler) ListAnalyticsSpendMetricsHandler(ctx echo.Context) error {
	var err error
	connectorTypes := source.ParseTypes(httpserver.QueryArrayParam(ctx, "connector"))
	connectionIDs, err := h.getConnectionIdFilterFromParams(ctx)
	if err != nil {
		return err
	}
	endTime, err := utils.TimeFromQueryParam(ctx, "endTime", time.Now())
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	startTime, err := utils.TimeFromQueryParam(ctx, "startTime", endTime.AddDate(0, -1, 0))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	pageNumber, pageSize, err := utils.PageConfigFromStrings(ctx.QueryParam("pageNumber"), ctx.QueryParam("pageSize"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, err.Error())
	}
	sortBy := strings.ToLower(ctx.QueryParam("sortBy"))
	if sortBy == "" {
		sortBy = "cost"
	}
	if sortBy != "dimension" && sortBy != "cost" &&
		sortBy != "growth" && sortBy != "growth_rate" {
		return ctx.JSON(http.StatusBadRequest, "invalid sortBy value")
	}

	costMetricMap := make(map[string]inventoryApi.CostMetric)
	if len(connectionIDs) > 0 {
		hits, err := es.FetchConnectionDailySpendHistoryByMetric(h.client, connectionIDs, connectorTypes, nil, startTime, endTime, EsFetchPageSize)
		if err != nil {
			return err
		}
		for _, hit := range hits {
			localHit := hit
			connector, _ := source.ParseType(localHit.Connector)
			if v, ok := costMetricMap[localHit.MetricID]; ok {
				exists := false
				for _, cnt := range v.Connector {
					if cnt.String() == connector.String() {
						exists = true
						break
					}
				}
				if !exists {
					v.Connector = append(v.Connector, connector)
				}
				v.TotalCost = utils.PAdd(v.TotalCost, &localHit.TotalCost)
				v.DailyCostAtStartTime = utils.PAdd(v.DailyCostAtStartTime, &localHit.StartDateCost)
				v.DailyCostAtEndTime = utils.PAdd(v.DailyCostAtEndTime, &localHit.EndDateCost)
				costMetricMap[localHit.MetricID] = v
			} else {
				costMetricMap[localHit.MetricID] = inventoryApi.CostMetric{
					Connector:            []source.Type{connector},
					CostDimensionName:    localHit.MetricName,
					TotalCost:            &localHit.TotalCost,
					DailyCostAtStartTime: &localHit.StartDateCost,
					DailyCostAtEndTime:   &localHit.EndDateCost,
				}
			}
		}
	} else {
		hits, err := es.FetchConnectorDailySpendHistoryByMetric(h.client, connectorTypes, nil, startTime, endTime, EsFetchPageSize)
		if err != nil {
			return err
		}
		for _, hit := range hits {
			localHit := hit
			connector, _ := source.ParseType(localHit.Connector)
			if v, ok := costMetricMap[localHit.MetricID]; ok {
				exists := false
				for _, cnt := range v.Connector {
					if cnt.String() == connector.String() {
						exists = true
						break
					}
				}
				if !exists {
					v.Connector = append(v.Connector, connector)
				}
				v.TotalCost = utils.PAdd(v.TotalCost, &localHit.TotalCost)
				v.DailyCostAtStartTime = utils.PAdd(v.DailyCostAtStartTime, &localHit.StartDateCost)
				v.DailyCostAtEndTime = utils.PAdd(v.DailyCostAtEndTime, &localHit.EndDateCost)
				costMetricMap[localHit.MetricID] = v
			} else {
				costMetricMap[localHit.MetricID] = inventoryApi.CostMetric{
					Connector:            []source.Type{connector},
					CostDimensionName:    localHit.MetricName,
					TotalCost:            &localHit.TotalCost,
					DailyCostAtStartTime: &localHit.StartDateCost,
					DailyCostAtEndTime:   &localHit.EndDateCost,
				}
			}
		}
	}

	var costMetrics []inventoryApi.CostMetric
	totalCost := float64(0)
	for _, costMetric := range costMetricMap {
		costMetrics = append(costMetrics, costMetric)
		if costMetric.TotalCost != nil {
			totalCost += *costMetric.TotalCost
		}
	}

	sort.Slice(costMetrics, func(i, j int) bool {
		switch sortBy {
		case "dimension":
			return costMetrics[i].CostDimensionName < costMetrics[j].CostDimensionName
		case "cost":
			if costMetrics[i].TotalCost == nil && costMetrics[j].TotalCost == nil {
				break
			}
			if costMetrics[i].TotalCost == nil {
				return false
			}
			if costMetrics[j].TotalCost == nil {
				return true
			}
			if *costMetrics[i].TotalCost != *costMetrics[j].TotalCost {
				return *costMetrics[i].TotalCost > *costMetrics[j].TotalCost
			}
		case "growth":
			diffi := utils.PSub(costMetrics[i].DailyCostAtEndTime, costMetrics[i].DailyCostAtStartTime)
			diffj := utils.PSub(costMetrics[j].DailyCostAtEndTime, costMetrics[j].DailyCostAtStartTime)
			if diffi == nil && diffj == nil {
				break
			}
			if diffi == nil {
				return false
			}
			if diffj == nil {
				return true
			}
			if *diffi != *diffj {
				return *diffi > *diffj
			}
		case "growth_rate":
			diffi := utils.PSub(costMetrics[i].DailyCostAtEndTime, costMetrics[i].DailyCostAtStartTime)
			diffj := utils.PSub(costMetrics[j].DailyCostAtEndTime, costMetrics[j].DailyCostAtStartTime)
			if diffi == nil && diffj == nil {
				break
			}
			if diffi == nil {
				return false
			}
			if diffj == nil {
				return true
			}
			if costMetrics[i].DailyCostAtStartTime == nil && costMetrics[j].DailyCostAtStartTime == nil {
				break
			}
			if costMetrics[i].DailyCostAtStartTime == nil {
				return true
			}
			if costMetrics[j].DailyCostAtStartTime == nil {
				return false
			}
			if *costMetrics[i].DailyCostAtStartTime == 0 && *costMetrics[j].DailyCostAtStartTime == 0 {
				break
			}
			if *costMetrics[i].DailyCostAtStartTime == 0 {
				return false
			}
			if *costMetrics[j].DailyCostAtStartTime == 0 {
				return true
			}
			if *diffi/(*costMetrics[i].DailyCostAtStartTime) != *diffj/(*costMetrics[j].DailyCostAtStartTime) {
				return *diffi/(*costMetrics[i].DailyCostAtStartTime) > *diffj/(*costMetrics[j].DailyCostAtStartTime)
			}
		}
		return costMetrics[i].CostDimensionName < costMetrics[j].CostDimensionName
	})

	return ctx.JSON(http.StatusOK, inventoryApi.ListCostMetricsResponse{
		TotalCount: len(costMetrics),
		TotalCost:  totalCost,
		Metrics:    utils.Paginate(pageNumber, pageSize, costMetrics),
	})
}

// ListAnalyticsSpendComposition godoc
//
//	@Summary		List cost composition
//	@Description	Retrieving the cost composition with respect to specified filters. Retrieving information such as the total cost for the given time range, and the top services by cost.
//	@Security		BearerToken
//	@Tags			analytics
//	@Accept			json
//	@Produce		json
//	@Param			connector		query		[]source.Type	false	"Connector type to filter by"
//	@Param			connectionId	query		[]string		false	"Connection IDs to filter by - mutually exclusive with connectionGroup"
//	@Param			connectionGroup	query		string			false	"Connection group to filter by - mutually exclusive with connectionId"
//	@Param			top				query		int				false	"How many top values to return default is 5"
//	@Param			startTime		query		string			false	"timestamp for start in epoch seconds"
//	@Param			endTime			query		string			false	"timestamp for end in epoch seconds"
//	@Success		200				{object}	inventoryApi.ListCostCompositionResponse
//	@Router			/inventory/api/v2/analytics/spend/composition [get]
func (h *HttpHandler) ListAnalyticsSpendComposition(ctx echo.Context) error {
	var err error
	connectorTypes := source.ParseTypes(httpserver.QueryArrayParam(ctx, "connector"))
	connectionIDs, err := h.getConnectionIdFilterFromParams(ctx)
	if err != nil {
		return err
	}
	endTime, err := utils.TimeFromQueryParam(ctx, "endTime", time.Now())
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	startTime, err := utils.TimeFromQueryParam(ctx, "startTime", endTime.AddDate(0, -1, 0))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	topStr := ctx.QueryParam("top")
	top := int64(5)
	if topStr != "" {
		top, err = strconv.ParseInt(topStr, 10, 64)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid top value")
		}
	}

	costMetricMap := make(map[string]inventoryApi.CostMetric)
	spends, err := es.FetchSpendByMetric(h.client, connectionIDs, connectorTypes, nil, startTime, endTime, EsFetchPageSize)
	if err != nil {
		return err
	}
	for metricID, spend := range spends {
		localSpend := spend
		costMetricMap[metricID] = inventoryApi.CostMetric{
			CostDimensionName: metricID,
			TotalCost:         &localSpend,
		}
	}

	var costMetrics []inventoryApi.CostMetric
	totalCost := float64(0)
	for _, costMetric := range costMetricMap {
		costMetrics = append(costMetrics, costMetric)
		if costMetric.TotalCost != nil {
			totalCost += *costMetric.TotalCost
		}
	}

	sort.Slice(costMetrics, func(i, j int) bool {
		if costMetrics[i].TotalCost == nil {
			return false
		}
		if costMetrics[j].TotalCost == nil {
			return true
		}
		if *costMetrics[i].TotalCost != *costMetrics[j].TotalCost {
			return *costMetrics[i].TotalCost > *costMetrics[j].TotalCost
		}
		return costMetrics[i].CostDimensionName < costMetrics[j].CostDimensionName
	})

	topCostMap := make(map[string]float64)
	othersCost := float64(0)
	if top > int64(len(costMetrics)) {
		top = int64(len(costMetrics))
	}
	for _, costMetric := range costMetrics[:int(top)] {
		if costMetric.TotalCost != nil {
			topCostMap[costMetric.CostDimensionName] = *costMetric.TotalCost
		}
	}
	if len(costMetrics) > int(top) {
		for _, costMetric := range costMetrics[int(top):] {
			if costMetric.TotalCost != nil {
				othersCost += *costMetric.TotalCost
			}
		}
	}

	return ctx.JSON(http.StatusOK, inventoryApi.ListCostCompositionResponse{
		TotalCount:     len(costMetrics),
		TotalCostValue: totalCost,
		TopValues:      topCostMap,
		Others:         othersCost,
	})
}

// GetAnalyticsSpendTrend godoc
//
//	@Summary		Get Cost Trend
//	@Description	Retrieving a list of costs over the course of the specified time frame based on the given input filters. If startTime and endTime are empty, the API returns the last month trend.
//	@Security		BearerToken
//	@Tags			analytics
//	@Accept			json
//	@Produce		json
//	@Param			connector		query		[]source.Type	false	"Connector type to filter by"
//	@Param			connectionId	query		[]string		false	"Connection IDs to filter by - mutually exclusive with connectionGroup"
//	@Param			metricIds		query		[]string		false	"Metrics IDs"
//	@Param			connectionGroup	query		string			false	"Connection group to filter by - mutually exclusive with connectionId"
//	@Param			startTime		query		string			false	"timestamp for start in epoch seconds"
//	@Param			endTime			query		string			false	"timestamp for end in epoch seconds"
//	@Param			datapointCount	query		string			false	"maximum number of datapoints to return, default is 30"
//	@Success		200				{object}	[]inventoryApi.CostTrendDatapoint
//	@Router			/inventory/api/v2/analytics/spend/trend [get]
func (h *HttpHandler) GetAnalyticsSpendTrend(ctx echo.Context) error {
	var err error
	metricIds := httpserver.QueryArrayParam(ctx, "metricIds")
	connectorTypes := source.ParseTypes(httpserver.QueryArrayParam(ctx, "connector"))
	connectionIDs, err := h.getConnectionIdFilterFromParams(ctx)
	if err != nil {
		return err
	}

	endTime, err := utils.TimeFromQueryParam(ctx, "endTime", time.Now())
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	startTime, err := utils.TimeFromQueryParam(ctx, "startTime", endTime.AddDate(0, -1, 0))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	datapointCountStr := ctx.QueryParam("datapointCount")
	datapointCount := int64(30)
	if datapointCountStr != "" {
		datapointCount, err = strconv.ParseInt(datapointCountStr, 10, 64)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid datapointCount")
		}
	}

	timepointToCost := map[string]float64{}
	if len(connectionIDs) > 0 {
		timepointToCost, err = es.FetchConnectionSpendTrend(h.client, metricIds, connectionIDs, connectorTypes, startTime, endTime)
	} else {
		timepointToCost, err = es.FetchConnectorSpendTrend(h.client, metricIds, connectorTypes, startTime, endTime)
	}
	if err != nil {
		return err
	}

	apiDatapoints := make([]inventoryApi.CostTrendDatapoint, 0, len(timepointToCost))
	for timeAt, costVal := range timepointToCost {
		dt, _ := time.Parse("2006-01-02", timeAt)
		apiDatapoints = append(apiDatapoints, inventoryApi.CostTrendDatapoint{Cost: costVal, Date: dt})
	}
	sort.Slice(apiDatapoints, func(i, j int) bool {
		return apiDatapoints[i].Date.Before(apiDatapoints[j].Date)
	})
	apiDatapoints = internal.DownSampleCostTrendDatapoints(apiDatapoints, int(datapointCount))

	return ctx.JSON(http.StatusOK, apiDatapoints)
}

// GetAnalyticsSpendMetricsTrend godoc
//
//	@Summary		Get Cost Trend
//	@Description	Retrieving a list of costs over the course of the specified time frame based on the given input filters. If startTime and endTime are empty, the API returns the last month trend.
//	@Security		BearerToken
//	@Tags			analytics
//	@Accept			json
//	@Produce		json
//	@Param			connector		query		[]source.Type	false	"Connector type to filter by"
//	@Param			connectionId	query		[]string		false	"Connection IDs to filter by - mutually exclusive with connectionGroup"
//	@Param			metricIds		query		[]string		false	"Metrics IDs"
//	@Param			connectionGroup	query		string			false	"Connection group to filter by - mutually exclusive with connectionId"
//	@Param			startTime		query		string			false	"timestamp for start in epoch seconds"
//	@Param			endTime			query		string			false	"timestamp for end in epoch seconds"
//	@Param			datapointCount	query		string			false	"maximum number of datapoints to return, default is 30"
//	@Success		200				{object}	[]inventoryApi.ListServicesCostTrendDatapoint
//	@Router			/inventory/api/v2/analytics/spend/metrics/trend [get]
func (h *HttpHandler) GetAnalyticsSpendMetricsTrend(ctx echo.Context) error {
	var err error
	metricIds := httpserver.QueryArrayParam(ctx, "metricIds")
	connectorTypes := source.ParseTypes(httpserver.QueryArrayParam(ctx, "connector"))
	connectionIDs, err := h.getConnectionIdFilterFromParams(ctx)
	if err != nil {
		return err
	}

	endTime, err := utils.TimeFromQueryParam(ctx, "endTime", time.Now())
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	startTime, err := utils.TimeFromQueryParam(ctx, "startTime", endTime.AddDate(0, -1, 0))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	datapointCountStr := ctx.QueryParam("datapointCount")
	datapointCount := int64(30)
	if datapointCountStr != "" {
		datapointCount, err = strconv.ParseInt(datapointCountStr, 10, 64)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid datapointCount")
		}
	}

	var mt []es.MetricTrend
	if len(connectionIDs) > 0 {
		mt, err = es.FetchConnectionSpendMetricTrend(h.client, metricIds, connectionIDs, connectorTypes, startTime, endTime)
	} else {
		mt, err = es.FetchConnectorSpendMetricTrend(h.client, metricIds, connectorTypes, startTime, endTime)
	}
	if err != nil {
		return err
	}

	var response []inventoryApi.ListServicesCostTrendDatapoint
	for _, m := range mt {
		apiDatapoints := make([]inventoryApi.CostTrendDatapoint, 0, len(m.Trend))
		for timeAt, costVal := range m.Trend {
			dt, _ := time.Parse("2006-01-02", timeAt)
			apiDatapoints = append(apiDatapoints, inventoryApi.CostTrendDatapoint{Cost: costVal, Date: dt})
		}
		sort.Slice(apiDatapoints, func(i, j int) bool {
			return apiDatapoints[i].Date.Before(apiDatapoints[j].Date)
		})
		apiDatapoints = internal.DownSampleCostTrendDatapoints(apiDatapoints, int(datapointCount))

		response = append(response, inventoryApi.ListServicesCostTrendDatapoint{
			ServiceName: m.MetricID,
			CostTrend:   apiDatapoints,
		})
	}

	return ctx.JSON(http.StatusOK, response)
}

// GetSpendTable godoc
//
//	@Summary		Get Spend Trend
//	@Description	Returns spend table with respect to the dimension and granularity
//	@Security		BearerToken
//	@Tags			inventory
//	@Accept			json
//	@Produce		json
//	@Param			startTime	query		string	false	"timestamp for start in epoch seconds"
//	@Param			endTime		query		string	false	"timestamp for end in epoch seconds"
//	@Param			granularity	query		string	false	"Granularity of the table, default is daily"	Enums(monthly, daily)
//	@Param			dimension	query		string	false	"Dimension of the table, default is metric"		Enums(connection, metric)
//
//	@Success		200			{object}	[]inventoryApi.SpendTableRow
//	@Router			/inventory/api/v2/analytics/spend/table [get]
func (h *HttpHandler) GetSpendTable(ctx echo.Context) error {
	var err error
	endTime, err := utils.TimeFromQueryParam(ctx, "endTime", time.Now())
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	startTime, err := utils.TimeFromQueryParam(ctx, "startTime", endTime.AddDate(0, -1, 0))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	granularity := inventoryApi.SpendTableGranularity(ctx.QueryParam("granularity"))
	if granularity != inventoryApi.SpendTableGranularityDaily &&
		granularity != inventoryApi.SpendTableGranularityMonthly {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid granularity")
	}
	dimension := inventoryApi.SpendDimension(ctx.QueryParam("dimension"))
	if dimension != inventoryApi.SpendDimensionMetric &&
		dimension != inventoryApi.SpendDimensionConnection {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid dimension")
	}

	mt, err := es.FetchSpendTableByDimension(h.client, dimension, startTime, endTime)
	if err != nil {
		return err
	}

	var table []inventoryApi.SpendTableRow
	for _, m := range mt {
		costValue := map[string]float64{}
		for dateKey, costItem := range m.Trend {
			dt, _ := time.Parse("2006-01-02", dateKey)
			monthKey := dt.Format("2006-01")
			if granularity == "daily" {
				costValue[dateKey] = costItem
			} else if granularity == "monthly" {
				if v, ok := costValue[monthKey]; ok {
					costValue[monthKey] = v + costItem
				} else {
					costValue[monthKey] = costItem
				}
			}
		}
		table = append(table, inventoryApi.SpendTableRow{
			DimensionID:   m.DimensionID,
			DimensionName: m.DimensionName,
			CostValue:     costValue,
		})
	}
	return ctx.JSON(http.StatusOK, table)
}

// GetResourceTypeMetricsHandler godoc
//
//	@Summary		List resource-type metrics
//	@Description	Retrieving metrics for a specific resource type.
//	@Security		BearerToken
//	@Tags			resource
//	@Accept			json
//	@Produce		json
//	@Param			connectionId	query		[]string	false	"Connection IDs to filter by - mutually exclusive with connectionGroup"
//	@Param			connectionGroup	query		string		false	"Connection group to filter by - mutually exclusive with connectionId"
//	@Param			endTime			query		string		false	"timestamp for resource count in epoch seconds"
//	@Param			startTime		query		string		false	"timestamp for resource count change comparison in epoch seconds"
//	@Param			resourceType	path		string		true	"ResourceType"
//	@Success		200				{object}	inventoryApi.ResourceType
//	@Router			/inventory/api/v2/resources/metric/{resourceType} [get]
func (h *HttpHandler) GetResourceTypeMetricsHandler(ctx echo.Context) error {
	var err error
	resourceType := ctx.Param("resourceType")
	connectionIDs, err := h.getConnectionIdFilterFromParams(ctx)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, err.Error())
	}
	if len(connectionIDs) > MaxConns {
		return ctx.JSON(http.StatusBadRequest, "too many connections")
	}
	endTimeStr := ctx.QueryParam("endTime")
	endTime := time.Now().Unix()
	if endTimeStr != "" {
		endTime, err = strconv.ParseInt(endTimeStr, 10, 64)
		if err != nil {
			return ctx.JSON(http.StatusBadRequest, "invalid endTime value")
		}
	}
	startTimeStr := ctx.QueryParam("startTime")
	startTime := time.Unix(endTime, 0).AddDate(0, 0, -7).Unix()
	if startTimeStr != "" {
		startTime, err = strconv.ParseInt(startTimeStr, 10, 64)
		if err != nil {
			return ctx.JSON(http.StatusBadRequest, "invalid startTime value")
		}
	}

	apiResourceType, err := h.GetResourceTypeMetric(resourceType, connectionIDs, endTime)
	if err != nil {
		return err
	}

	oldApiResourceType, err := h.GetResourceTypeMetric(resourceType, connectionIDs, startTime)
	if err != nil {
		return err
	}
	apiResourceType.OldCount = oldApiResourceType.Count

	return ctx.JSON(http.StatusOK, *apiResourceType)
}

func (h *HttpHandler) GetResourceTypeMetric(resourceTypeStr string, connectionIDs []string, timeAt int64) (*inventoryApi.ResourceType, error) {
	resourceType, err := h.db.GetResourceType(resourceTypeStr)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, echo.NewHTTPError(http.StatusNotFound, "resource type not found")
		}
		return nil, err
	}

	var metricIndexed map[string]int
	if len(connectionIDs) > 0 {
		metricIndexed, err = es.FetchConnectionResourceTypeCountAtTime(h.client, nil, connectionIDs, time.Unix(timeAt, 0), []string{resourceTypeStr}, EsFetchPageSize)
	} else {
		metricIndexed, err = es.FetchConnectorResourceTypeCountAtTime(h.client, nil, time.Unix(timeAt, 0), []string{resourceTypeStr}, EsFetchPageSize)
	}
	if err != nil {
		return nil, err
	}

	apiResourceType := resourceType.ToApi()
	if count, ok := metricIndexed[strings.ToLower(resourceType.ResourceType)]; ok {
		apiResourceType.Count = &count
	}

	return &apiResourceType, nil
}

func (h *HttpHandler) ListConnectionsData(ctx echo.Context) error {
	performanceStartTime := time.Now()
	var err error
	connectionIDs := httpserver.QueryArrayParam(ctx, "connectionId")
	connectors, err := h.getConnectorTypesFromConnectionIDs(ctx, nil, connectionIDs)
	if err != nil {
		return err
	}
	endTimeStr := ctx.QueryParam("endTime")
	endTime := time.Now()
	if endTimeStr != "" {
		endTimeUnix, err := strconv.ParseInt(endTimeStr, 10, 64)
		if err != nil {
			return ctx.JSON(http.StatusBadRequest, "endTime is not a valid integer")
		}
		endTime = time.Unix(endTimeUnix, 0)
	}
	startTimeStr := ctx.QueryParam("startTime")
	startTime := endTime.AddDate(0, 0, -7)
	if startTimeStr != "" {
		startTimeUnix, err := strconv.ParseInt(startTimeStr, 10, 64)
		if err != nil {
			return ctx.JSON(http.StatusBadRequest, "startTime is not a valid integer")
		}
		startTime = time.Unix(startTimeUnix, 0)
	}
	needCostStr := ctx.QueryParam("needCost")
	needCost := true
	if needCostStr == "false" {
		needCost = false
	}
	needResourceCountStr := ctx.QueryParam("needResourceCount")
	needResourceCount := true
	if needResourceCountStr == "false" {
		needResourceCount = false
	}

	fmt.Println("ListConnectionsData part1 ", time.Now().Sub(performanceStartTime).Milliseconds())
	res := map[string]inventoryApi.ConnectionData{}
	if needResourceCount {
		resourceCounts, err := es.FetchConnectionAnalyticsResourcesCountAtTime(h.client, connectors, connectionIDs, endTime, EsFetchPageSize)
		if err != nil {
			return err
		}
		for _, hit := range resourceCounts {
			localHit := hit
			if _, ok := res[localHit.ConnectionID.String()]; !ok {
				res[localHit.ConnectionID.String()] = inventoryApi.ConnectionData{
					ConnectionID: localHit.ConnectionID.String(),
				}
			}
			v := res[localHit.ConnectionID.String()]
			v.Count = utils.PAdd(v.Count, &localHit.ResourceCount)
			if v.LastInventory == nil || v.LastInventory.IsZero() || v.LastInventory.Before(time.UnixMilli(localHit.EvaluatedAt)) {
				v.LastInventory = utils.GetPointer(time.UnixMilli(localHit.EvaluatedAt))
			}
			res[localHit.ConnectionID.String()] = v
		}
		fmt.Println("ListConnectionsData part2 ", time.Now().Sub(performanceStartTime).Milliseconds())
		oldResourceCount, err := es.FetchConnectionAnalyticsResourcesCountAtTime(h.client, connectors, connectionIDs, startTime, EsFetchPageSize)
		if err != nil {
			return err
		}
		for _, hit := range oldResourceCount {
			localHit := hit
			if _, ok := res[localHit.ConnectionID.String()]; !ok {
				res[localHit.ConnectionID.String()] = inventoryApi.ConnectionData{
					ConnectionID:  localHit.ConnectionID.String(),
					LastInventory: nil,
				}
			}
			v := res[localHit.ConnectionID.String()]
			v.OldCount = utils.PAdd(v.OldCount, &localHit.ResourceCount)
			if v.LastInventory == nil || v.LastInventory.IsZero() || v.LastInventory.Before(time.UnixMilli(localHit.EvaluatedAt)) {
				v.LastInventory = utils.GetPointer(time.UnixMilli(localHit.EvaluatedAt))
			}
			res[localHit.ConnectionID.String()] = v
		}
		fmt.Println("ListConnectionsData part3 ", time.Now().Sub(performanceStartTime).Milliseconds())
	}

	if needCost {
		hits, err := es.FetchConnectionDailySpendHistoryByMetric(h.client, connectionIDs, connectors, nil, startTime, endTime, EsFetchPageSize)
		if err != nil {
			return err
		}
		for _, hit := range hits {
			localHit := hit
			if v, ok := res[localHit.ConnectionID]; ok {
				v.TotalCost = utils.PAdd(v.TotalCost, &localHit.TotalCost)
				v.DailyCostAtStartTime = utils.PAdd(v.DailyCostAtStartTime, &localHit.StartDateCost)
				v.DailyCostAtEndTime = utils.PAdd(v.DailyCostAtEndTime, &localHit.EndDateCost)
				res[localHit.ConnectionID] = v
			} else {
				res[localHit.ConnectionID] = inventoryApi.ConnectionData{
					ConnectionID:         localHit.ConnectionID,
					Count:                nil,
					OldCount:             nil,
					LastInventory:        nil,
					TotalCost:            &localHit.TotalCost,
					DailyCostAtStartTime: &localHit.StartDateCost,
					DailyCostAtEndTime:   &localHit.EndDateCost,
				}
			}
		}
		fmt.Println("ListConnectionsData part4 ", time.Now().Sub(performanceStartTime).Milliseconds())
	}

	return ctx.JSON(http.StatusOK, res)
}

func (h *HttpHandler) GetConnectionData(ctx echo.Context) error {
	connectionId := ctx.Param("connectionId")
	endTimeStr := ctx.QueryParam("endTime")
	endTime := time.Now()
	if endTimeStr != "" {
		endTimeUnix, err := strconv.ParseInt(endTimeStr, 10, 64)
		if err != nil {
			return ctx.JSON(http.StatusBadRequest, "endTime is not a valid integer")
		}
		endTime = time.Unix(endTimeUnix, 0)
	}
	startTimeStr := ctx.QueryParam("startTime")
	startTime := endTime.AddDate(0, 0, -7)
	if startTimeStr != "" {
		startTimeUnix, err := strconv.ParseInt(startTimeStr, 10, 64)
		if err != nil {
			return ctx.JSON(http.StatusBadRequest, "startTime is not a valid integer")
		}
		startTime = time.Unix(startTimeUnix, 0)
	}

	res := inventoryApi.ConnectionData{
		ConnectionID: connectionId,
	}

	resourceCounts, err := es.FetchConnectionAnalyticsResourcesCountAtTime(h.client, nil, []string{connectionId}, endTime, EsFetchPageSize)
	for _, hit := range resourceCounts {
		if hit.ConnectionID.String() != connectionId {
			continue
		}
		localHit := hit
		res.Count = utils.PAdd(res.Count, &localHit.ResourceCount)
		if res.LastInventory == nil || res.LastInventory.IsZero() || res.LastInventory.Before(time.UnixMilli(localHit.EvaluatedAt)) {
			res.LastInventory = utils.GetPointer(time.UnixMilli(localHit.EvaluatedAt))
		}
	}

	oldResourceCounts, err := es.FetchConnectionAnalyticsResourcesCountAtTime(h.client, nil, []string{connectionId}, startTime, EsFetchPageSize)
	for _, hit := range oldResourceCounts {
		if hit.ConnectionID.String() != connectionId {
			continue
		}
		localHit := hit
		res.OldCount = utils.PAdd(res.OldCount, &localHit.ResourceCount)
		if res.LastInventory == nil || res.LastInventory.IsZero() || res.LastInventory.Before(time.UnixMilli(localHit.EvaluatedAt)) {
			res.LastInventory = utils.GetPointer(time.UnixMilli(localHit.EvaluatedAt))
		}
	}

	costs, err := es.FetchDailyCostHistoryByAccountsBetween(h.client, nil, []string{connectionId}, endTime, startTime, EsFetchPageSize)
	if err != nil {
		return err
	}
	startTimeCosts, err := es.FetchDailyCostHistoryByAccountsAtTime(h.client, nil, []string{connectionId}, startTime)
	if err != nil {
		return err
	}
	endTimeCosts, err := es.FetchDailyCostHistoryByAccountsAtTime(h.client, nil, []string{connectionId}, endTime)
	if err != nil {
		return err
	}

	for costConnectionId, costValue := range costs {
		if costConnectionId != connectionId {
			continue
		}
		localValue := costValue
		res.TotalCost = utils.PAdd(res.TotalCost, &localValue)
	}
	for costConnectionId, costValue := range startTimeCosts {
		if costConnectionId != connectionId {
			continue
		}
		localValue := costValue
		res.DailyCostAtStartTime = utils.PAdd(res.DailyCostAtStartTime, &localValue)
	}
	for costConnectionId, costValue := range endTimeCosts {
		if costConnectionId != connectionId {
			continue
		}
		localValue := costValue
		res.DailyCostAtEndTime = utils.PAdd(res.DailyCostAtEndTime, &localValue)
	}

	return ctx.JSON(http.StatusOK, res)
}

// ListQueries godoc
//
//	@Summary		List smart queries
//	@Description	Retrieving list of smart queries by specified filters
//	@Security		BearerToken
//	@Tags			smart_query
//	@Produce		json
//	@Param			request	body		inventoryApi.ListQueryRequest	true	"Request Body"
//	@Success		200		{object}	[]inventoryApi.SmartQueryItem
//	@Router			/inventory/api/v1/query [get]
func (h *HttpHandler) ListQueries(ctx echo.Context) error {
	var req inventoryApi.ListQueryRequest
	if err := bindValidate(ctx, &req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	var search *string
	if len(req.TitleFilter) > 0 {
		search = &req.TitleFilter
	}

	queries, err := h.db.GetQueriesWithFilters(search, req.Connectors)
	if err != nil {
		return err
	}

	var result []inventoryApi.SmartQueryItem
	for _, item := range queries {
		category := ""

		result = append(result, inventoryApi.SmartQueryItem{
			ID:          item.Model.ID,
			Provider:    item.Connector,
			Title:       item.Title,
			Category:    category,
			Description: item.Description,
			Query:       item.Query,
			Tags:        nil,
		})
	}
	return ctx.JSON(200, result)
}

// RunQuery godoc
//
//	@Summary		Run query
//	@Description	Run provided smart query and returns the result.
//	@Security		BearerToken
//	@Tags			smart_query
//	@Accepts		json
//	@Produce		json
//	@Param			request	body		inventoryApi.RunQueryRequest	true	"Request Body"
//	@Param			accept	header		string							true	"Accept header"	Enums(application/json,text/csv)
//	@Success		200		{object}	inventoryApi.RunQueryResponse
//	@Router			/inventory/api/v1/query/run [post]
func (h *HttpHandler) RunQuery(ctx echo.Context) error {
	var req inventoryApi.RunQueryRequest
	if err := bindValidate(ctx, &req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if req.Query == nil || *req.Query == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Query is required")
	}
	resp, err := h.RunSmartQuery(ctx.Request().Context(), *req.Query, *req.Query, &req)
	if err != nil {
		return err
	}
	return ctx.JSON(200, resp)
}

// GetRecentRanQueries godoc
//
//	@Summary		List recently ran queries
//	@Description	List queries which have been run recently
//	@Security		BearerToken
//	@Tags			smart_query
//	@Accepts		json
//	@Produce		json
//	@Success		200	{object}	[]inventoryApi.SmartQueryHistory
//	@Router			/inventory/api/v1/query/run/history [get]
func (h *HttpHandler) GetRecentRanQueries(ctx echo.Context) error {
	smartQueryHistories, err := h.db.GetQueryHistory()
	if err != nil {
		h.logger.Error("Failed to get query history", zap.Error(err))
		return err
	}

	res := make([]inventoryApi.SmartQueryHistory, 0, len(smartQueryHistories))
	for _, history := range smartQueryHistories {
		res = append(res, history.ToApi())
	}

	return ctx.JSON(200, res)
}

func (h *HttpHandler) CountResources(ctx echo.Context) error {
	timeAt := time.Now()
	resourceTypes, err := h.db.ListFilteredResourceTypes(nil, nil, nil, nil, true)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	resourceTypeNames := make([]string, 0, len(resourceTypes))
	for _, resourceType := range resourceTypes {
		resourceTypeNames = append(resourceTypeNames, resourceType.ResourceType)
	}

	metricsIndexed, err := es.FetchConnectorResourceTypeCountAtTime(h.client, nil, timeAt, resourceTypeNames, EsFetchPageSize)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	totalCount := 0
	for _, count := range metricsIndexed {
		totalCount += count
	}
	return ctx.JSON(http.StatusOK, totalCount)
}

func (h *HttpHandler) RunSmartQuery(ctx context.Context, title, query string, req *inventoryApi.RunQueryRequest) (*inventoryApi.RunQueryResponse, error) {
	var err error
	lastIdx := (req.Page.No - 1) * req.Page.Size

	direction := inventoryApi.DirectionType("")
	orderBy := ""
	if req.Sorts != nil && len(req.Sorts) > 0 {
		direction = req.Sorts[0].Direction
		orderBy = req.Sorts[0].Field
	}
	if len(req.Sorts) > 1 {
		return nil, errors.New("multiple sort items not supported")
	}

	h.logger.Info("executing smart query", zap.String("query", query))
	res, err := h.steampipeConn.Query(ctx, query, &lastIdx, &req.Page.Size, orderBy, steampipe.DirectionType(direction))
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	err = h.db.UpdateQueryHistory(query)
	if err != nil {
		h.logger.Error("failed to update query history", zap.Error(err))
		return nil, err
	}

	resp := inventoryApi.RunQueryResponse{
		Title:   title,
		Query:   query,
		Headers: res.Headers,
		Result:  res.Data,
	}
	return &resp, nil
}

func (h *HttpHandler) ListInsightResults(ctx echo.Context) error {
	var err error
	connectors := source.ParseTypes(httpserver.QueryArrayParam(ctx, "connector"))
	timeStr := ctx.QueryParam("time")
	timeAt := time.Now().Unix()
	if timeStr != "" {
		timeAt, err = strconv.ParseInt(timeStr, 10, 64)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid time")
		}
	}
	connectionIDs := httpserver.QueryArrayParam(ctx, "connectionId")

	insightIdListStr := httpserver.QueryArrayParam(ctx, "insightId")
	if len(insightIdListStr) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "insight id is required")
	}
	insightIdList := make([]uint, 0, len(insightIdListStr))
	for _, idStr := range insightIdListStr {
		id, err := strconv.ParseUint(idStr, 10, 32)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid insight id")
		}
		insightIdList = append(insightIdList, uint(id))
	}

	var insightValues map[uint][]insight.InsightResource
	if timeStr != "" {
		insightValues, err = es.FetchInsightValueAtTime(h.client, time.Unix(timeAt, 0), connectors, connectionIDs, insightIdList, true)
	} else {
		insightValues, err = es.FetchInsightValueAtTime(h.client, time.Unix(timeAt, 0), connectors, connectionIDs, insightIdList, false)
	}
	if err != nil {
		return err
	}

	firstAvailable, err := es.FetchInsightValueAfter(h.client, time.Unix(timeAt, 0), connectors, connectionIDs, insightIdList)
	if err != nil {
		return err
	}

	for insightId, _ := range firstAvailable {
		if results, ok := insightValues[insightId]; ok && len(results) > 0 {
			continue
		}
		insightValues[insightId] = firstAvailable[insightId]
	}

	return ctx.JSON(http.StatusOK, insightValues)
}

func (h *HttpHandler) GetInsightResult(ctx echo.Context) error {
	insightId, err := strconv.ParseUint(ctx.Param("insightId"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid insight id")
	}
	timeStr := ctx.QueryParam("time")
	timeAt := time.Now().Unix()
	if timeStr != "" {
		timeAt, err = strconv.ParseInt(timeStr, 10, 64)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid time")
		}
	}
	connectionIDs := httpserver.QueryArrayParam(ctx, "connectionId")
	if len(connectionIDs) == 0 {
		connectionIDs = nil
	}

	var insightResults map[uint][]insight.InsightResource
	if timeStr != "" {
		insightResults, err = es.FetchInsightValueAtTime(h.client, time.Unix(timeAt, 0), nil, connectionIDs, []uint{uint(insightId)}, true)
	} else {
		insightResults, err = es.FetchInsightValueAtTime(h.client, time.Unix(timeAt, 0), nil, connectionIDs, []uint{uint(insightId)}, false)
	}
	if err != nil {
		return err
	}

	firstAvailable, err := es.FetchInsightValueAfter(h.client, time.Unix(timeAt, 0), nil, connectionIDs, []uint{uint(insightId)})
	if err != nil {
		return err
	}

	for insightId, _ := range firstAvailable {
		if results, ok := insightResults[insightId]; ok && len(results) > 0 {
			continue
		}
		insightResults[insightId] = firstAvailable[insightId]
	}

	if insightResult, ok := insightResults[uint(insightId)]; ok {
		return ctx.JSON(http.StatusOK, insightResult)
	} else {
		return echo.NewHTTPError(http.StatusNotFound, "no data for insight found")
	}
}

func (h *HttpHandler) GetInsightTrendResults(ctx echo.Context) error {
	insightId, err := strconv.ParseUint(ctx.Param("insightId"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid insight id")
	}
	var startTime, endTime time.Time
	endTime = time.Now()
	if timeStr := ctx.QueryParam("endTime"); timeStr != "" {
		timeInt, err := strconv.ParseInt(timeStr, 10, 64)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid time")
		}
		endTime = time.Unix(timeInt, 0)
	}
	if timeStr := ctx.QueryParam("startTime"); timeStr != "" {
		timeInt, err := strconv.ParseInt(timeStr, 10, 64)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid time")
		}
		startTime = time.Unix(timeInt, 0)
	} else {
		startTime = endTime.Add(-time.Hour * 24 * 30)
	}

	connectionIDs := httpserver.QueryArrayParam(ctx, "connectionId")

	dataPointCount := int(endTime.Sub(startTime).Hours() / 24)
	insightResults, err := es.FetchInsightAggregatedPerQueryValuesBetweenTimes(h.client, startTime, endTime, dataPointCount, nil, connectionIDs, []uint{uint(insightId)})
	if err != nil {
		return err
	}
	if insightResult, ok := insightResults[uint(insightId)]; ok {
		return ctx.JSON(http.StatusOK, insightResult)
	} else {
		return echo.NewHTTPError(http.StatusNotFound, "no data for insight found")
	}
}

func (h *HttpHandler) ListResourceTypeMetadata(ctx echo.Context) error {
	tagMap := model.TagStringsToTagMap(httpserver.QueryArrayParam(ctx, "tag"))
	connectors := source.ParseTypes(httpserver.QueryArrayParam(ctx, "connector"))
	serviceNames := httpserver.QueryArrayParam(ctx, "service")
	resourceTypeNames := httpserver.QueryArrayParam(ctx, "resourceType")
	summarized := strings.ToLower(ctx.QueryParam("summarized")) == "true"
	pageNumber, pageSize, err := utils.PageConfigFromStrings(ctx.QueryParam("pageNumber"), ctx.QueryParam("pageSize"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, err.Error())
	}
	resourceTypes, err := h.db.ListFilteredResourceTypes(tagMap, resourceTypeNames, serviceNames, connectors, summarized)
	if err != nil {
		return err
	}

	var resourceTypeMetadata []inventoryApi.ResourceType
	tableCountMap := make(map[string]int)
	insightList, err := h.complianceClient.ListInsightsMetadata(httpclient.FromEchoContext(ctx), connectors)
	if err != nil {
		return err
	}
	for _, insightEntity := range insightList {
		for _, insightTable := range insightEntity.Query.ListOfTables {
			tableCountMap[insightTable]++
		}
	}

	for _, resourceType := range resourceTypes {
		apiResourceType := resourceType.ToApi()

		var table string
		switch resourceType.Connector {
		case source.CloudAWS:
			table = awsSteampipe.ExtractTableName(resourceType.ResourceType)
		case source.CloudAzure:
			table = azureSteampipe.ExtractTableName(resourceType.ResourceType)
		}
		insightTableCount := 0
		if table != "" {
			insightTableCount = tableCountMap[table]
		}
		apiResourceType.InsightsCount = utils.GetPointerOrNil(insightTableCount)

		// TODO: add compliance count

		resourceTypeMetadata = append(resourceTypeMetadata, apiResourceType)
	}

	sort.Slice(resourceTypeMetadata, func(i, j int) bool {
		return resourceTypeMetadata[i].ResourceType < resourceTypeMetadata[j].ResourceType
	})

	result := inventoryApi.ListResourceTypeMetadataResponse{
		TotalResourceTypeCount: len(resourceTypeMetadata),
		ResourceTypes:          utils.Paginate(pageNumber, pageSize, resourceTypeMetadata),
	}

	return ctx.JSON(http.StatusOK, result)
}
