package describer

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	logstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
)

func CloudWatchAlarm(ctx context.Context, cfg aws.Config) ([]interface{}, error) {
	client := cloudwatch.NewFromConfig(cfg)
	paginator := cloudwatch.NewDescribeAlarmsPaginator(client, &cloudwatch.DescribeAlarmsInput{
		AlarmTypes: []types.AlarmType{types.AlarmTypeMetricAlarm},
	})

	var values []interface{}
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, v := range page.MetricAlarms {
			values = append(values, v)
		}
	}

	return values, nil
}

func CloudWatchAnomalyDetector(ctx context.Context, cfg aws.Config) ([]interface{}, error) {
	client := cloudwatch.NewFromConfig(cfg)
	output, err := client.DescribeAnomalyDetectors(ctx, &cloudwatch.DescribeAnomalyDetectorsInput{})
	if err != nil {
		return nil, err
	}

	var values []interface{}
	for _, v := range output.AnomalyDetectors {
		values = append(values, v)
	}

	return values, nil
}

func CloudWatchCompositeAlarm(ctx context.Context, cfg aws.Config) ([]interface{}, error) {
	client := cloudwatch.NewFromConfig(cfg)
	paginator := cloudwatch.NewDescribeAlarmsPaginator(client, &cloudwatch.DescribeAlarmsInput{
		AlarmTypes: []types.AlarmType{types.AlarmTypeCompositeAlarm},
	})

	var values []interface{}
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, v := range page.MetricAlarms {
			values = append(values, v)
		}
	}

	return values, nil
}

func CloudWatchDashboard(ctx context.Context, cfg aws.Config) ([]interface{}, error) {
	client := cloudwatch.NewFromConfig(cfg)
	output, err := client.ListDashboards(ctx, &cloudwatch.ListDashboardsInput{})
	if err != nil {
		return nil, err
	}

	var values []interface{}
	for _, v := range output.DashboardEntries {
		values = append(values, v)
	}

	return values, nil
}

func CloudWatchInsightRule(ctx context.Context, cfg aws.Config) ([]interface{}, error) {
	client := cloudwatch.NewFromConfig(cfg)
	paginator := cloudwatch.NewDescribeInsightRulesPaginator(client, &cloudwatch.DescribeInsightRulesInput{})

	var values []interface{}
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, v := range page.InsightRules {
			values = append(values, v)
		}
	}

	return values, nil
}

func CloudWatchMetricStream(ctx context.Context, cfg aws.Config) ([]interface{}, error) {
	client := cloudwatch.NewFromConfig(cfg)
	output, err := client.ListMetricStreams(ctx, &cloudwatch.ListMetricStreamsInput{})
	if err != nil {
		return nil, err
	}

	var values []interface{}
	for _, v := range output.Entries {
		values = append(values, v)
	}

	return values, nil
}

func CloudWatchLogsDestination(ctx context.Context, cfg aws.Config) ([]interface{}, error) {
	client := cloudwatchlogs.NewFromConfig(cfg)
	paginator := cloudwatchlogs.NewDescribeDestinationsPaginator(client, &cloudwatchlogs.DescribeDestinationsInput{})

	var values []interface{}
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, v := range page.Destinations {
			values = append(values, v)
		}
	}

	return values, nil
}

func CloudWatchLogsLogGroup(ctx context.Context, cfg aws.Config) ([]interface{}, error) {
	client := cloudwatchlogs.NewFromConfig(cfg)
	paginator := cloudwatchlogs.NewDescribeLogGroupsPaginator(client, &cloudwatchlogs.DescribeLogGroupsInput{})

	var values []interface{}
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, v := range page.LogGroups {
			values = append(values, v)
		}
	}

	return values, nil
}

func CloudWatchLogsLogStream(ctx context.Context, cfg aws.Config) ([]interface{}, error) {
	logGroups, err := CloudWatchLogsLogGroup(ctx, cfg)
	if err != nil {
		return nil, err
	}

	var values []interface{}
	for _, logGroup := range logGroups {
		client := cloudwatchlogs.NewFromConfig(cfg)
		paginator := cloudwatchlogs.NewDescribeLogStreamsPaginator(client, &cloudwatchlogs.DescribeLogStreamsInput{
			LogGroupName: logGroup.(logstypes.LogGroup).LogGroupName,
		})

		for paginator.HasMorePages() {
			page, err := paginator.NextPage(ctx)
			if err != nil {
				return nil, err
			}

			for _, v := range page.LogStreams {
				values = append(values, v)
			}
		}
	}

	return values, nil
}

func CloudWatchLogsMetricFilter(ctx context.Context, cfg aws.Config) ([]interface{}, error) {
	client := cloudwatchlogs.NewFromConfig(cfg)
	paginator := cloudwatchlogs.NewDescribeMetricFiltersPaginator(client, &cloudwatchlogs.DescribeMetricFiltersInput{})

	var values []interface{}
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, v := range page.MetricFilters {
			values = append(values, v)
		}
	}

	return values, nil
}

func CloudWatchLogsQueryDefinition(ctx context.Context, cfg aws.Config) ([]interface{}, error) {
	client := cloudwatchlogs.NewFromConfig(cfg)

	var values []interface{}
	err := PaginateRetrieveAll(func(prevToken *string) (nextToken *string, err error) {
		output, err := client.DescribeQueryDefinitions(ctx, &cloudwatchlogs.DescribeQueryDefinitionsInput{NextToken: prevToken})
		if err != nil {
			return nil, err
		}

		for _, v := range output.QueryDefinitions {
			values = append(values, v)
		}

		return output.NextToken, nil
	})
	if err != nil {
		return nil, err
	}

	return values, nil
}

func CloudWatchLogsResourcePolicy(ctx context.Context, cfg aws.Config) ([]interface{}, error) {
	client := cloudwatchlogs.NewFromConfig(cfg)

	var values []interface{}
	err := PaginateRetrieveAll(func(prevToken *string) (nextToken *string, err error) {
		output, err := client.DescribeResourcePolicies(ctx, &cloudwatchlogs.DescribeResourcePoliciesInput{NextToken: prevToken})
		if err != nil {
			return nil, err
		}

		for _, v := range output.ResourcePolicies {
			values = append(values, v)
		}

		return output.NextToken, nil
	})
	if err != nil {
		return nil, err
	}

	return values, nil
}

func CloudWatchLogsSubscriptionFilter(ctx context.Context, cfg aws.Config) ([]interface{}, error) {
	logGroups, err := CloudWatchLogsLogGroup(ctx, cfg)
	if err != nil {
		return nil, err
	}

	var values []interface{}
	for _, logGroup := range logGroups {
		client := cloudwatchlogs.NewFromConfig(cfg)
		paginator := cloudwatchlogs.NewDescribeSubscriptionFiltersPaginator(client, &cloudwatchlogs.DescribeSubscriptionFiltersInput{
			LogGroupName: logGroup.(logstypes.LogGroup).LogGroupName,
		})

		for paginator.HasMorePages() {
			page, err := paginator.NextPage(ctx)
			if err != nil {
				return nil, err
			}

			for _, v := range page.SubscriptionFilters {
				values = append(values, v)
			}
		}
	}

	return values, nil
}
