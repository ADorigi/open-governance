package calculator

import (
	"encoding/json"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v4"
	"github.com/kaytu-io/kaytu-engine/pkg/cost-estimator/es"
	"io"
	"net/http"
	"strings"
	"time"
)

func VirtualMachineCostEstimator(OSType *armcompute.OperatingSystemTypes, armRegionName *string, armSkuName *armcompute.VirtualMachineSizeTypes) (float64, error) {
	serviceName := "Virtual Machines"
	typeN := "Consumption"
	serviceFamily := "Compute"

	filter := fmt.Sprintf("serviceName eq '%v' and type eq '%v' and serviceFamily eq '%v' and armSkuName eq '%v' and armRegionName eq '%v' ", serviceName, typeN, serviceFamily, armSkuName, armRegionName)
	req, err := http.NewRequest("GET", "https://prices.azure.com/api/retail/prices", nil)
	if err != nil {
		return 0, fmt.Errorf("error in request to azure for giving the cost : %v ", err)
	}
	q := req.URL.Query()
	q.Add("$filter", filter)
	req.URL.RawQuery = q.Encode()

	client := http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("error in sending the request : %v ", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("error status equal to : %v ", res.StatusCode)
	}

	responseBody, err := io.ReadAll(res.Body)
	if err != nil {
		return 0, fmt.Errorf("error in read the response : %v ", err)
	}

	var response es.AzureCostStr
	err = json.Unmarshal(responseBody, &response)
	if err != nil {
		return 0, fmt.Errorf("error in unmarshalling the response : %v ", err)
	}
	OSTypeS := string(*OSType)
	item, err := giveProperCostTime(response.Items, OSTypeS)
	if err != nil {
		return 0, err
	}

	return item.RetailPrice, nil
}

func giveProperCostTime(Items []es.ItemsStr, OSType string) (es.ItemsStr, error) {
	newTime := 1
	var newItem es.ItemsStr
	osTypeCheckWindows := true
	if OSType == "Linux" {
		osTypeCheckWindows = false
	}

	for i := 0; i < len(Items); i++ {
		item := Items[i]

		checkOsType := strings.Contains(item.ProductName, "Windows")
		if osTypeCheckWindows {
			if !checkOsType {
				continue
			}
		} else {
			if checkOsType {
				continue
			}
		}

		timeP, err := time.Parse(time.RFC3339, item.EffectiveStartDate)
		if err != nil {
			return es.ItemsStr{}, fmt.Errorf("error in parsing time : %v ", err)
		}

		if timeP.Year() > newTime {
			newTime = timeP.Year()
			newItem = Items[i]
		}
	}

	return newItem, nil
}
