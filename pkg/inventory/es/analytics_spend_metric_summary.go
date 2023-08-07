package es

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/kaytu-io/kaytu-engine/pkg/analytics/es/spend"
	"github.com/kaytu-io/kaytu-engine/pkg/summarizer/es"
	"github.com/kaytu-io/kaytu-util/pkg/keibi-es-sdk"
	"github.com/kaytu-io/kaytu-util/pkg/source"
	"strconv"
	"time"
)

type MetricTrend struct {
	MetricID string
	Trend    map[string]float64
}

type ConnectionSpendMetricTrendQueryResponse struct {
	Aggregations struct {
		MetricIDGroup struct {
			Buckets []struct {
				Key       string `json:"key"`
				DateGroup struct {
					Buckets []struct {
						Key          string `json:"key"`
						CostSumGroup struct {
							Value float64 `json:"value"`
						} `json:"cost_sum_group"`
					} `json:"buckets"`
				} `json:"date_group"`
			} `json:"buckets"`
		} `json:"metric_id_group"`
	} `json:"aggregations"`
}

func FetchConnectionSpendMetricTrend(client keibi.Client, metricIds []string, connectionIDs []string, connectors []source.Type, startTime, endTime time.Time) ([]MetricTrend, error) {
	query := make(map[string]any)
	var filters []any

	if len(connectionIDs) > 0 {
		filters = append(filters, map[string]any{
			"terms": map[string][]string{"connection_id": connectionIDs},
		})
	}
	if len(metricIds) > 0 {
		filters = append(filters, map[string]any{
			"terms": map[string][]string{"metric_id": metricIds},
		})
	}
	if len(connectors) > 0 {
		filters = append(filters, map[string]any{
			"terms": map[string][]source.Type{"connector": connectors},
		})
	}
	filters = append(filters, map[string]any{
		"range": map[string]any{
			"period_end": map[string]string{
				"lte": strconv.FormatInt(endTime.UnixMilli(), 10),
			},
		},
	})
	filters = append(filters, map[string]any{
		"range": map[string]any{
			"period_start": map[string]string{
				"gte": strconv.FormatInt(startTime.UnixMilli(), 10),
			},
		},
	})

	query["size"] = 0
	query["query"] = map[string]any{
		"bool": map[string]any{
			"filter": filters,
		},
	}
	query["aggs"] = map[string]any{
		"metric_id_group": map[string]any{
			"terms": map[string]any{
				"field": "metric_id",
				"size":  es.EsFetchPageSize,
			},
			"aggs": map[string]any{
				"date_group": map[string]any{
					"terms": map[string]any{
						"field": "date",
						"size":  es.EsFetchPageSize,
					},
					"aggs": map[string]any{
						"cost_sum_group": map[string]any{
							"sum": map[string]string{
								"field": "cost_value",
							},
						},
					},
				},
			},
		},
	}

	queryJson, err := json.Marshal(query)
	if err != nil {
		return nil, err
	}
	fmt.Printf("FetchConnectionSpendTrend = %s\n", queryJson)

	var response ConnectionSpendMetricTrendQueryResponse
	err = client.Search(context.Background(), spend.AnalyticsSpendConnectionSummaryIndex, string(queryJson), &response)
	if err != nil {
		return nil, err
	}

	var result []MetricTrend
	for _, bucket := range response.Aggregations.MetricIDGroup.Buckets {
		mt := MetricTrend{
			MetricID: bucket.Key,
			Trend:    make(map[string]float64),
		}
		for _, dateBucket := range bucket.DateGroup.Buckets {
			mt.Trend[dateBucket.Key] = dateBucket.CostSumGroup.Value
		}
		result = append(result, mt)
	}

	return result, nil
}

type ConnectorSpendMetricTrendQueryResponse struct {
	Aggregations struct {
		MetricIDGroup struct {
			Buckets []struct {
				Key       string `json:"key"`
				DateGroup struct {
					Buckets []struct {
						Key          string `json:"key"`
						CostSumGroup struct {
							Value float64 `json:"value"`
						} `json:"cost_sum_group"`
					} `json:"buckets"`
				} `json:"date_group"`
			} `json:"buckets"`
		} `json:"metric_id_group"`
	} `json:"aggregations"`
}

func FetchConnectorSpendMetricTrend(client keibi.Client, metricIds []string, connectors []source.Type, startTime, endTime time.Time) ([]MetricTrend, error) {
	query := make(map[string]any)
	var filters []any

	if len(connectors) > 0 {
		filters = append(filters, map[string]any{
			"terms": map[string][]source.Type{"connector": connectors},
		})
	}
	if len(metricIds) > 0 {
		filters = append(filters, map[string]any{
			"terms": map[string][]string{"metric_id": metricIds},
		})
	}
	filters = append(filters, map[string]any{
		"range": map[string]any{
			"period_end": map[string]string{
				"lte": strconv.FormatInt(endTime.UnixMilli(), 10),
			},
		},
	})
	filters = append(filters, map[string]any{
		"range": map[string]any{
			"period_start": map[string]string{
				"gte": strconv.FormatInt(startTime.UnixMilli(), 10),
			},
		},
	})

	query["size"] = 0
	query["query"] = map[string]any{
		"bool": map[string]any{
			"filter": filters,
		},
	}
	query["aggs"] = map[string]any{
		"metric_id_group": map[string]any{
			"terms": map[string]any{
				"field": "metric_id",
				"size":  es.EsFetchPageSize,
			},
			"aggs": map[string]any{
				"date_group": map[string]any{
					"terms": map[string]any{
						"field": "date",
						"size":  es.EsFetchPageSize,
					},
					"aggs": map[string]any{
						"cost_sum_group": map[string]any{
							"sum": map[string]string{
								"field": "cost_value",
							},
						},
					},
				},
			},
		},
	}

	queryJson, err := json.Marshal(query)
	if err != nil {
		return nil, err
	}
	fmt.Printf("FetchConnectorSpendTrend = %s\n", queryJson)

	var response ConnectorSpendMetricTrendQueryResponse
	err = client.Search(context.Background(), spend.AnalyticsSpendConnectorSummaryIndex, string(queryJson), &response)
	if err != nil {
		return nil, err
	}

	var result []MetricTrend
	for _, bucket := range response.Aggregations.MetricIDGroup.Buckets {
		mt := MetricTrend{
			MetricID: bucket.Key,
			Trend:    make(map[string]float64),
		}
		for _, dateBucket := range bucket.DateGroup.Buckets {
			mt.Trend[dateBucket.Key] = dateBucket.CostSumGroup.Value
		}
		result = append(result, mt)
	}

	return result, nil
}
