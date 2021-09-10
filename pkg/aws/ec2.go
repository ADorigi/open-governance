package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

func getEC2Instances(ctx context.Context, cfg aws.Config) ([]interface{}, error) {
	client := ec2.NewFromConfig(cfg)
	paginator := ec2.NewDescribeInstancesPaginator(client, &ec2.DescribeInstancesInput{MaxResults: aws.Int32(1000)})

	var instances []interface{}
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, r := range page.Reservations {
			for _, v := range r.Instances {
				instances = append(instances, v)
			}
		}
	}

	return instances, nil
}

func getEC2NatGateways(ctx context.Context, cfg aws.Config) ([]interface{}, error) {
	client := ec2.NewFromConfig(cfg)
	paginator := ec2.NewDescribeNatGatewaysPaginator(client, &ec2.DescribeNatGatewaysInput{MaxResults: aws.Int32(pagingSize)})

	var gateways []interface{}
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, v := range page.NatGateways {
			gateways = append(gateways, v)
		}
	}

	return gateways, nil
}

func getEC2Routes(ctx context.Context, cfg aws.Config) ([]interface{}, error) {
	client := ec2.NewFromConfig(cfg)
	paginator := ec2.NewDescribeRouteTablesPaginator(client, &ec2.DescribeRouteTablesInput{MaxResults: aws.Int32(routeTablePageSize)})

	var routes []interface{}
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, t := range page.RouteTables {
			for _, v := range t.Routes {
				routes = append(routes, v)
			}
		}
	}

	return routes, nil
}

func getEC2RouteTables(ctx context.Context, cfg aws.Config) ([]interface{}, error) {
	client := ec2.NewFromConfig(cfg)
	paginator := ec2.NewDescribeRouteTablesPaginator(client, &ec2.DescribeRouteTablesInput{MaxResults: aws.Int32(routeTablePageSize)})

	var routeTables []interface{}
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, v := range page.RouteTables {
			routeTables = append(routeTables, v)
		}
	}

	return routeTables, nil
}

func getEC2SecurityGroups(ctx context.Context, cfg aws.Config) ([]interface{}, error) {
	client := ec2.NewFromConfig(cfg)
	paginator := ec2.NewDescribeSecurityGroupsPaginator(client, &ec2.DescribeSecurityGroupsInput{MaxResults: aws.Int32(pagingSize)})

	var securityGroups []interface{}
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, v := range page.SecurityGroups {
			securityGroups = append(securityGroups, v)
		}
	}

	return securityGroups, nil
}

func getEC2Subnets(ctx context.Context, cfg aws.Config) ([]interface{}, error) {
	client := ec2.NewFromConfig(cfg)
	paginator := ec2.NewDescribeSubnetsPaginator(client, &ec2.DescribeSubnetsInput{MaxResults: aws.Int32(pagingSize)})

	var subnets []interface{}
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, v := range page.Subnets {
			subnets = append(subnets, v)
		}
	}

	return subnets, nil
}

func getEC2TransitGateways(ctx context.Context, cfg aws.Config) ([]interface{}, error) {
	client := ec2.NewFromConfig(cfg)
	paginator := ec2.NewDescribeTransitGatewaysPaginator(client, &ec2.DescribeTransitGatewaysInput{MaxResults: aws.Int32(pagingSize)})

	var gateways []interface{}
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, v := range page.TransitGateways {
			gateways = append(gateways, v)
		}
	}

	return gateways, nil
}

func getEC2TransitGatewayAttachments(ctx context.Context, cfg aws.Config) ([]interface{}, error) {
	client := ec2.NewFromConfig(cfg)
	paginator := ec2.NewDescribeTransitGatewayAttachmentsPaginator(client, &ec2.DescribeTransitGatewayAttachmentsInput{MaxResults: aws.Int32(pagingSize)})

	var attachments []interface{}
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, v := range page.TransitGatewayAttachments {
			attachments = append(attachments, v)
		}
	}

	return attachments, nil
}

func getEC2TransitGatewayConnets(ctx context.Context, cfg aws.Config) ([]interface{}, error) {
	client := ec2.NewFromConfig(cfg)
	paginator := ec2.NewDescribeTransitGatewayConnectsPaginator(client, &ec2.DescribeTransitGatewayConnectsInput{MaxResults: aws.Int32(pagingSize)})

	var connects []interface{}
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, v := range page.TransitGatewayConnects {
			connects = append(connects, v)
		}
	}

	return connects, nil
}

func getEC2Volumes(ctx context.Context, cfg aws.Config) ([]interface{}, error) {
	client := ec2.NewFromConfig(cfg)
	paginator := ec2.NewDescribeVolumesPaginator(client, &ec2.DescribeVolumesInput{MaxResults: aws.Int32(pagingSize)})

	var volumes []interface{}
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, v := range page.Volumes {
			volumes = append(volumes, v)
		}
	}

	return volumes, nil
}

func getEC2VolumeAttachments(ctx context.Context, cfg aws.Config) ([]interface{}, error) {
	client := ec2.NewFromConfig(cfg)
	paginator := ec2.NewDescribeVolumesPaginator(client, &ec2.DescribeVolumesInput{MaxResults: aws.Int32(pagingSize)})

	var attachments []interface{}
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, v := range page.Volumes {
			for _, a := range v.Attachments {
				attachments = append(attachments, a)
			}
		}
	}

	return attachments, nil
}

func getEC2Vpcs(ctx context.Context, cfg aws.Config) ([]interface{}, error) {
	client := ec2.NewFromConfig(cfg)
	paginator := ec2.NewDescribeVpcsPaginator(client, &ec2.DescribeVpcsInput{MaxResults: aws.Int32(pagingSize)})

	var vpcs []interface{}
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, v := range page.Vpcs {
			vpcs = append(vpcs, v)
		}
	}

	return vpcs, nil

}

func getEC2VpcPeeringConnections(ctx context.Context, cfg aws.Config) ([]interface{}, error) {
	client := ec2.NewFromConfig(cfg)
	paginator := ec2.NewDescribeVpcPeeringConnectionsPaginator(client, &ec2.DescribeVpcPeeringConnectionsInput{MaxResults: aws.Int32(pagingSize)})

	var connections []interface{}
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, v := range page.VpcPeeringConnections {
			connections = append(connections, v)
		}
	}

	return connections, nil
}
