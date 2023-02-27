package es

import (
	"fmt"
	"strconv"
	"strings"

	"gitlab.com/keibiengine/keibi-engine/pkg/source"
)

const (
	ProviderSummaryIndex = "provider_summary"
)

type ProviderReportType string

const (
	ServiceProviderSummary           ProviderReportType = "ServicePerProvider"
	CategoryProviderSummary          ProviderReportType = "CategoryPerProvider"
	ResourceTypeProviderSummary      ProviderReportType = "ResourceTypePerProvider"
	LocationProviderSummary          ProviderReportType = "LocationPerProvider"
	TrendProviderSummary             ProviderReportType = "TrendPerProviderHistory"
	ResourceTypeTrendProviderSummary ProviderReportType = "ResourceTypeTrendPerProviderHistory"
	CostProviderSummaryMonthly       ProviderReportType = "CostPerService"
	CostProviderSummaryDaily         ProviderReportType = "CostPerServiceDaily"
)

type ProviderServiceSummary struct {
	SummarizeJobID   uint               `json:"summarize_job_id"`
	ScheduleJobID    uint               `json:"schedule_job_id"`
	ServiceName      string             `json:"service_name"`
	ResourceType     string             `json:"resource_type"`
	SourceType       source.Type        `json:"source_type"`
	DescribedAt      int64              `json:"described_at"`
	ResourceCount    int                `json:"resource_count"`
	LastDayCount     *int               `json:"last_day_count"`
	LastWeekCount    *int               `json:"last_week_count"`
	LastQuarterCount *int               `json:"last_quarter_count"`
	LastYearCount    *int               `json:"last_year_count"`
	ReportType       ProviderReportType `json:"report_type"`
}

func (r ProviderServiceSummary) KeysAndIndex() ([]string, string) {
	keys := []string{
		r.ServiceName,
		r.SourceType.String(),
		string(ServiceProviderSummary),
	}
	if strings.HasSuffix(string(r.ReportType), "History") {
		keys = append(keys, fmt.Sprintf("%d", r.SummarizeJobID))
	}
	return keys, ProviderSummaryIndex
}

type ProviderCategorySummary struct {
	SummarizeJobID   uint               `json:"summarize_job_id"`
	ScheduleJobID    uint               `json:"schedule_job_id"`
	CategoryName     string             `json:"category_name"`
	ResourceType     string             `json:"resource_type"`
	SourceType       source.Type        `json:"source_type"`
	DescribedAt      int64              `json:"described_at"`
	ResourceCount    int                `json:"resource_count"`
	LastDayCount     *int               `json:"last_day_count"`
	LastWeekCount    *int               `json:"last_week_count"`
	LastQuarterCount *int               `json:"last_quarter_count"`
	LastYearCount    *int               `json:"last_year_count"`
	ReportType       ProviderReportType `json:"report_type"`
}

func (r ProviderCategorySummary) KeysAndIndex() ([]string, string) {
	keys := []string{
		r.CategoryName,
		r.SourceType.String(),
		string(CategoryProviderSummary),
	}
	if strings.HasSuffix(string(r.ReportType), "History") {
		keys = append(keys, fmt.Sprintf("%d", r.SummarizeJobID))
	}
	return keys, ProviderSummaryIndex
}

type ProviderTrendSummary struct {
	SummarizeJobID uint               `json:"summarize_job_id"`
	ScheduleJobID  uint               `json:"schedule_job_id"`
	SourceType     source.Type        `json:"source_type"`
	DescribedAt    int64              `json:"described_at"`
	ResourceCount  int                `json:"resource_count"`
	ReportType     ProviderReportType `json:"report_type"`
}

func (r ProviderTrendSummary) KeysAndIndex() ([]string, string) {
	keys := []string{
		r.SourceType.String(),
		strconv.FormatInt(r.DescribedAt, 10),
		string(TrendProviderSummary),
	}
	if strings.HasSuffix(string(r.ReportType), "History") {
		keys = append(keys, fmt.Sprintf("%d", r.SummarizeJobID))
	}
	return keys, ProviderSummaryIndex
}

type ProviderResourceTypeTrendSummary struct {
	SummarizeJobID uint               `json:"summarize_job_id"`
	ScheduleJobID  uint               `json:"schedule_job_id"`
	SourceType     source.Type        `json:"source_type"`
	DescribedAt    int64              `json:"described_at"`
	ResourceType   string             `json:"resource_type"`
	ResourceCount  int                `json:"resource_count"`
	ReportType     ProviderReportType `json:"report_type"`
}

func (r ProviderResourceTypeTrendSummary) KeysAndIndex() ([]string, string) {
	keys := []string{
		r.SourceType.String(),
		r.ResourceType,
		strconv.FormatInt(r.DescribedAt, 10),
		string(ResourceTypeTrendProviderSummary),
	}
	if strings.HasSuffix(string(r.ReportType), "History") {
		keys = append(keys, fmt.Sprintf("%d", r.SummarizeJobID))
	}
	return keys, ProviderSummaryIndex
}

type ProviderResourceTypeSummary struct {
	SummarizeJobID   uint               `json:"summarize_job_id"`
	ScheduleJobID    uint               `json:"schedule_job_id"`
	ResourceType     string             `json:"resource_type"`
	SourceType       source.Type        `json:"source_type"`
	ResourceCount    int                `json:"resource_count"`
	LastDayCount     *int               `json:"last_day_count"`
	LastWeekCount    *int               `json:"last_week_count"`
	LastQuarterCount *int               `json:"last_quarter_count"`
	LastYearCount    *int               `json:"last_year_count"`
	ReportType       ProviderReportType `json:"report_type"`
}

func (r ProviderResourceTypeSummary) KeysAndIndex() ([]string, string) {
	keys := []string{
		r.ResourceType,
		r.SourceType.String(),
		string(ResourceTypeProviderSummary),
	}
	if strings.HasSuffix(string(r.ReportType), "History") {
		keys = append(keys, fmt.Sprintf("%d", r.SummarizeJobID))
	}
	return keys, ProviderSummaryIndex
}

type ProviderLocationSummary struct {
	SummarizeJobID       uint               `json:"summarize_job_id"`
	ScheduleJobID        uint               `json:"schedule_job_id"`
	SourceType           source.Type        `json:"source_type"`
	LocationDistribution map[string]int     `json:"location_distribution"`
	ReportType           ProviderReportType `json:"report_type"`
}

func (r ProviderLocationSummary) KeysAndIndex() ([]string, string) {
	keys := []string{
		r.SourceType.String(),
		string(LocationProviderSummary),
	}
	if strings.HasSuffix(string(r.ReportType), "History") {
		keys = append(keys, fmt.Sprintf("%d", r.SummarizeJobID))
	}
	return keys, ProviderSummaryIndex
}
