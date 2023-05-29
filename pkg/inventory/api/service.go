package api

import (
	"time"

	"github.com/kaytu-io/kaytu-util/pkg/source"
)

type Service struct {
	Connector         source.Type         `json:"connector"`
	ServiceName       string              `json:"service_name"`
	ServiceLabel      string              `json:"service_label"`
	Tags              map[string][]string `json:"tags,omitempty"`
	LogoURI           *string             `json:"logo_uri,omitempty"`
	Cost              *float64            `json:"count,omitempty"`
	CostChangePercent *float64            `json:"count_change_percent,omitempty"`
}

type ListServiceMetricsResponse struct {
	TotalCost     float64   `json:"total_cost"`
	TotalServices int       `json:"total_services"`
	Services      []Service `json:"services"`
}

type ListServiceCostCompositionResponse struct {
	TotalCost       float64            `json:"total_count"`
	TotalValueCount int                `json:"total_value_count"`
	TopValues       map[string]float64 `json:"top_values"`
	Others          float64            `json:"others"`
}

type CostTrendDatapoint struct {
	Cost float64   `json:"count"`
	Date time.Time `json:"date"`
}
