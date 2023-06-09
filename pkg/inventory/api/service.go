package api

import (
	"time"

	"github.com/kaytu-io/kaytu-util/pkg/source"
)

type Service struct {
	Connector       source.Type         `json:"connector"`
	ServiceName     string              `json:"service_name"`
	ServiceLabel    string              `json:"service_label"`
	ResourceTypes   []ResourceType      `json:"resource_types"`
	IsCostSupported bool                `json:"is_cost_supported"`
	Tags            map[string][]string `json:"tags,omitempty"`
	LogoURI         *string             `json:"logo_uri,omitempty"`

	ResourceCount    *int     `json:"resource_count,omitempty"`
	OldResourceCount *int     `json:"old_resource_count,omitempty"`
	Cost             *float64 `json:"cost,omitempty"`
	StartDailyCost   *float64 `json:"start_daily_cost,omitempty"`
	EndDailyCost     *float64 `json:"end_daily_cost,omitempty"`
}

type ListServiceMetricsResponse struct {
	TotalCost     float64   `json:"total_cost"`
	TotalServices int       `json:"total_services"`
	Services      []Service `json:"services"`
}

type ListServiceMetadataResponse struct {
	TotalServiceCount int       `json:"total_service_count"`
	Services          []Service `json:"services"`
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
