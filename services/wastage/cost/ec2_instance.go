package cost

import (
	"encoding/json"
	"fmt"
	types2 "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/kaytu-io/kaytu-engine/pkg/httpclient"
	kaytu_client "github.com/kaytu-io/kaytu-engine/pkg/steampipe-plugin-kaytu/kaytu-client"
	"github.com/kaytu-io/kaytu-engine/services/wastage/api/entity"
	"github.com/kaytu-io/pennywise/pkg/cost"
	"github.com/kaytu-io/pennywise/pkg/schema"
	"net/http"
	"time"
)

func (s *Service) GetEC2InstanceCost(region string, instance entity.EC2Instance, volumes []entity.EC2Volume, metrics map[string][]types2.Datapoint) (float64, error) {
	req := schema.Submission{
		ID:        "submission-1",
		CreatedAt: time.Now(),
		Resources: []schema.ResourceDef{},
	}

	valuesMap := map[string]any{}
	valuesMap["instance_type"] = instance.InstanceType
	if instance.Placement != nil {
		valuesMap["tenancy"] = instance.Placement.Tenancy
		valuesMap["availability_zone"] = instance.Placement.AvailabilityZone
		valuesMap["host_id"] = instance.Placement.HashedHostId
	}
	valuesMap["ebs_optimized"] = instance.EbsOptimized
	if instance.Monitoring != nil {
		if *instance.Monitoring == "disabled" || *instance.Monitoring == "disabling" {
			valuesMap["monitoring"] = false
		} else {
			valuesMap["monitoring"] = true
		}
	}
	//if instance.CpuOptions != nil {
	//	valuesMap["credit_specification"] = []map[string]any{{
	//		"cpu_credits": *instance.CpuOptions, //TODO - not sure
	//	}}
	//}
	var blockDevices []map[string]any
	for _, v := range volumes {
		blockDevices = append(blockDevices, map[string]any{
			"device_name": v.HashedVolumeId,
			"volume_type": v.VolumeType,
			"volume_size": *v.Size,
			"iops":        *v.Iops,
		})
	}
	valuesMap["ebs_block_device"] = blockDevices
	valuesMap["launch_template"] = []map[string]any{}
	if instance.InstanceLifecycle == types.InstanceLifecycleTypeSpot {
		valuesMap["spot_price"] = "Spot"
	} else {
		valuesMap["spot_price"] = ""
	}

	os := "Linux"
	if instance.Platform != "" {
		os = instance.Platform
	}
	valuesMap["pennywise_usage"] = map[string]any{
		"operating_system": os,
		"operation":        instance.UsageOperation,
		//"reserved_instance_type": "",
		//"reserved_instance_term": "",
		//"reserved_instance_payment_option": "",
		//"monthly_cpu_credit_hrs": "",
		//"vcpu_count": "",
		"monthly_hrs": "730",
	}

	req.Resources = append(req.Resources, schema.ResourceDef{
		Address:      instance.HashedInstanceId,
		Type:         kaytu_client.ResourceTypeConversion("aws::ec2::instance"),
		Name:         "",
		RegionCode:   region,
		ProviderName: schema.AWSProvider,
		Values:       valuesMap,
	})

	reqBody, err := json.Marshal(req)
	if err != nil {
		return 0, err
	}

	var response cost.State
	statusCode, err := httpclient.DoRequest("GET", s.pennywiseBaseUrl+"/api/v1/cost/submission", nil, reqBody, &response)
	if err != nil {
		return 0, err
	}

	if statusCode != http.StatusOK {
		return 0, fmt.Errorf("failed to get pennywise cost, status code = %d", statusCode)
	}

	resourceCost, err := response.Cost()
	if err != nil {
		return 0, err
	}

	return resourceCost.Decimal.InexactFloat64(), nil
}

func (s *Service) GetEBSVolumeCost(region string, volume entity.EC2Volume, volumeMetrics map[string][]types2.Datapoint) (float64, error) {
	req := schema.Submission{
		ID:        "submission-1",
		CreatedAt: time.Now(),
		Resources: []schema.ResourceDef{},
	}

	valuesMap := map[string]any{}
	valuesMap["availability_zone"] = *volume.AvailabilityZone
	valuesMap["type"] = volume.VolumeType
	valuesMap["size"] = *volume.Size
	valuesMap["iops"] = *volume.Iops
	valuesMap["throughput"] = volume.Throughput

	req.Resources = append(req.Resources, schema.ResourceDef{
		Address:      volume.HashedVolumeId,
		Type:         kaytu_client.ResourceTypeConversion("aws::ec2::volume"),
		Name:         "",
		RegionCode:   region,
		ProviderName: schema.AWSProvider,
		Values:       valuesMap,
	})

	reqBody, err := json.Marshal(req)
	if err != nil {
		return 0, err
	}

	var response cost.State
	statusCode, err := httpclient.DoRequest("GET", s.pennywiseBaseUrl+"/api/v1/cost/submission", nil, reqBody, &response)
	if err != nil {
		return 0, err
	}

	if statusCode != http.StatusOK {
		return 0, fmt.Errorf("failed to get pennywise cost, status code = %d", statusCode)
	}

	resourceCost, err := response.Cost()
	if err != nil {
		return 0, err
	}

	return resourceCost.Decimal.InexactFloat64(), nil
}

func (s *Service) EstimateLicensePrice(region string, instance entity.EC2Instance) (float64, error) {
	withLicense, err := s.GetEC2InstanceCost(region, instance, nil, nil)
	if err != nil {
		return 0, err
	}
	fmt.Println("=========================================")
	fmt.Println("With License Price: ", instance, withLicense)
	fmt.Println("=========================================")
	instance.UsageOperation = mapLicenseToNoLicense[instance.UsageOperation]
	withoutLicense, err := s.GetEC2InstanceCost(region, instance, nil, nil)
	if err != nil {
		return 0, err
	}
	fmt.Println("=========================================")
	fmt.Println("WithOUT License Price: ", instance, withoutLicense)
	fmt.Println("=========================================")
	return withLicense - withoutLicense, nil
}

var mapLicenseToNoLicense = map[string]string{
	// Red Hat
	"RunInstances:00g0": "RunInstances:00g0",
	"RunInstances:0010": "RunInstances:00g0",
	"RunInstances:1010": "RunInstances:00g0",
	"RunInstances:1014": "RunInstances:00g0",
	"RunInstances:1110": "RunInstances:00g0",
	"RunInstances:0014": "RunInstances:00g0",
	"RunInstances:0210": "RunInstances:00g0",
	"RunInstances:0110": "RunInstances:00g0",
	// Windows
	"RunInstances:0002": "RunInstances:0800",
	"RunInstances:0800": "RunInstances:0800",
	"RunInstances:0102": "RunInstances:0800",
	"RunInstances:0006": "RunInstances:0800",
	"RunInstances:0202": "RunInstances:0800",
	// Linux/UNIX
	"RunInstances":      "RunInstances",
	"RunInstances:0004": "RunInstances",
	"RunInstances:0200": "RunInstances",
	"RunInstances:000g": "RunInstances",
	"RunInstances:0g00": "RunInstances",
	"RunInstances:0100": "RunInstances",
}
