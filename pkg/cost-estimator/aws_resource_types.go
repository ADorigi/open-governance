package cost_estimator

import (
	"fmt"
	apiAuth "github.com/kaytu-io/kaytu-engine/pkg/auth/api"
	"github.com/kaytu-io/kaytu-engine/pkg/cost-estimator/es"
	"github.com/kaytu-io/kaytu-engine/pkg/internal/httpclient"
)

func GetEC2InstanceCost(h *HttpHandler, resourceId string) (float64, error) {
	var response es.EC2InstanceResponse
	resource, err := es.GetElasticsearch(h.client, resourceId, "AWS::EC2::Instance", response)
	if err != nil {
		return 0, err
	}
	if len(response.Hits.Hits) == 0 {
		return 0, fmt.Errorf("no resource found")
	}
	cost, err := h.workspaceClient.GetEC2InstanceCost(&httpclient.Context{UserRole: apiAuth.InternalRole}, resource.(es.EC2InstanceResponse))
	if err != nil {
		return 0, err
	}

	return cost, nil
}

func GetEC2VolumeCost(h *HttpHandler, resourceId string) (float64, error) {
	var response es.EC2VolumeResponse
	resource, err := es.GetElasticsearch(h.client, resourceId, "AWS::EC2::Volume", response)
	if err != nil {
		return 0, err
	}
	if len(response.Hits.Hits) == 0 {
		return 0, fmt.Errorf("no resource found")
	}
	cost, err := h.workspaceClient.GetEC2VolumeCost(&httpclient.Context{UserRole: apiAuth.InternalRole}, resource.(es.EC2VolumeResponse))
	if err != nil {
		return 0, err
	}

	return cost, nil
}

func GetELBCost(h *HttpHandler, resourceId string) (float64, error) {
	var response es.EC2VolumeResponse
	resource, err := es.GetElasticsearch(h.client, resourceId, "AWS::ElasticLoadBalancingV2::LoadBalancer", response)
	if err != nil {
		return 0, err
	}
	if len(response.Hits.Hits) == 0 {
		return 0, fmt.Errorf("no resource found")
	} else {

	}
	cost, err := h.workspaceClient.GetEC2VolumeCost(&httpclient.Context{UserRole: apiAuth.InternalRole}, resource.(es.EC2VolumeResponse))
	if err != nil {
		return 0, err
	}

	return cost, nil
}
