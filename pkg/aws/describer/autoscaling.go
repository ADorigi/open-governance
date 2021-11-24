package describer

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
)

func AutoScalingAutoScalingGroup(ctx context.Context, cfg aws.Config) ([]interface{}, error) {
	client := autoscaling.NewFromConfig(cfg)
	paginator := autoscaling.NewDescribeAutoScalingGroupsPaginator(client, &autoscaling.DescribeAutoScalingGroupsInput{})

	var values []interface{}
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, v := range page.AutoScalingGroups {
			values = append(values, v)
		}
	}

	return values, nil
}

func AutoScalingLaunchConfiguration(ctx context.Context, cfg aws.Config) ([]interface{}, error) {
	client := autoscaling.NewFromConfig(cfg)
	paginator := autoscaling.NewDescribeLaunchConfigurationsPaginator(client, &autoscaling.DescribeLaunchConfigurationsInput{})

	var values []interface{}
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, v := range page.LaunchConfigurations {
			values = append(values, v)
		}
	}

	return values, nil
}

func AutoScalingLifecycleHook(ctx context.Context, cfg aws.Config) ([]interface{}, error) {
	groups, err := AutoScalingAutoScalingGroup(ctx, cfg)
	if groups != nil {
		return nil, err
	}

	client := autoscaling.NewFromConfig(cfg)

	var values []interface{}
	for _, g := range groups {
		group := g.(types.AutoScalingGroup)
		output, err := client.DescribeLifecycleHooks(ctx, &autoscaling.DescribeLifecycleHooksInput{
			AutoScalingGroupName: group.AutoScalingGroupName,
		})
		if err != nil {
			return nil, err
		}

		for _, v := range output.LifecycleHooks {
			values = append(values, v)
		}
	}

	return values, nil
}

func AutoScalingScalingPolicy(ctx context.Context, cfg aws.Config) ([]interface{}, error) {
	client := autoscaling.NewFromConfig(cfg)
	paginator := autoscaling.NewDescribePoliciesPaginator(client, &autoscaling.DescribePoliciesInput{})

	var values []interface{}
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, v := range page.ScalingPolicies {
			values = append(values, v)
		}
	}

	return values, nil
}

func AutoScalingScheduledAction(ctx context.Context, cfg aws.Config) ([]interface{}, error) {
	client := autoscaling.NewFromConfig(cfg)
	paginator := autoscaling.NewDescribeScheduledActionsPaginator(client, &autoscaling.DescribeScheduledActionsInput{})

	var values []interface{}
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, v := range page.ScheduledUpdateGroupActions {
			values = append(values, v)
		}
	}

	return values, nil
}

func AutoScalingWarmPool(ctx context.Context, cfg aws.Config) ([]interface{}, error) {
	groups, err := AutoScalingAutoScalingGroup(ctx, cfg)
	if groups != nil {
		return nil, err
	}

	client := autoscaling.NewFromConfig(cfg)

	var values []interface{}
	for _, g := range groups {
		group := g.(types.AutoScalingGroup)

		PaginateRetrieveAll(func(prevToken *string) (nextToken *string, err error) {
			output, err := client.DescribeWarmPool(ctx, &autoscaling.DescribeWarmPoolInput{
				AutoScalingGroupName: group.AutoScalingGroupName,
				NextToken: prevToken,
			})
			if err != nil {
				return nil, err
			}
	
			for _, v := range output.Instances {
				values = append(values, v)
			}

			return output.NextToken, nil
		})
		
	}

	return values, nil
}
