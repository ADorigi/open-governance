package inventory

import (
	"fmt"

	"github.com/go-redis/cache/v8"
	"github.com/go-redis/redis/v8"
	"gitlab.com/keibiengine/keibi-engine/pkg/describe/kafka"

	"gitlab.com/keibiengine/keibi-engine/pkg/cloudservice"

	"gitlab.com/keibiengine/keibi-engine/pkg/inventory/api"
	"gitlab.com/keibiengine/keibi-engine/pkg/inventory/es"
	"gitlab.com/keibiengine/keibi-engine/pkg/keibi-es-sdk"
	"gitlab.com/keibiengine/keibi-engine/pkg/source"
)

func GetCategories(client keibi.Client, rcache *redis.Client, cache *cache.Cache,
	provider source.Type, sourceID *string) ([]api.CategoriesResponse, error) {
	var hits []kafka.SourceCategorySummary

	var providerPtr *string
	if provider != "" {
		v := string(provider)
		providerPtr = &v
	}

	if cached, err := es.FetchCategoriesCached(rcache, cache, providerPtr, sourceID); err == nil && len(cached) > 0 {
		hits = cached
		fmt.Println("fetching categories from cached")
	} else {
		res, err := es.GetCategoriesQuery(client, string(provider), sourceID)
		if err != nil {
			return nil, err
		}
		hits = res
		fmt.Println("fetching categories from ES")
	}

	categoryMap := map[string]api.CategoriesResponse{}
	for _, hit := range hits {
		if v, ok := categoryMap[hit.CategoryName]; ok {
			v.ResourceCount += hit.ResourceCount
			categoryMap[hit.CategoryName] = v
		} else {
			categoryMap[hit.CategoryName] = api.CategoriesResponse{
				CategoryName:     hit.CategoryName,
				ResourceCount:    hit.ResourceCount,
				LastDayCount:     hit.LastDayCount,
				LastWeekCount:    hit.LastWeekCount,
				LastQuarterCount: hit.LastQuarterCount,
				LastYearCount:    hit.LastYearCount,
			}
		}
	}

	var res []api.CategoriesResponse
	for _, v := range categoryMap {
		res = append(res, v)
	}

	return res, nil
}

func GetServices(client keibi.Client, rcache *redis.Client, cache *cache.Cache,
	provider source.Type, sourceID *string) ([]api.TopServicesResponse, error) {
	var hits []kafka.SourceServicesSummary

	var providerPtr *string
	if provider != "" {
		v := string(provider)
		providerPtr = &v
	}

	if cached, err := es.FetchServicesCached(rcache, cache, providerPtr, sourceID); err == nil && len(cached) > 0 {
		hits = cached
		fmt.Println("fetching services from cached")
	} else {
		res, err := es.FetchServicesQuery(client, string(provider), sourceID)
		if err != nil {
			return nil, err
		}
		hits = res
		fmt.Println("fetching services from ES")
	}

	serviceResponse := map[string]api.TopServicesResponse{}
	for _, hit := range hits {
		if v, ok := serviceResponse[hit.ServiceName]; ok {
			v.ResourceCount += hit.ResourceCount
			serviceResponse[hit.ServiceName] = v
		} else {
			serviceResponse[hit.ServiceName] = api.TopServicesResponse{
				ServiceName:      hit.ServiceName,
				Provider:         string(hit.SourceType),
				ResourceCount:    hit.ResourceCount,
				LastDayCount:     hit.LastDayCount,
				LastWeekCount:    hit.LastWeekCount,
				LastQuarterCount: hit.LastQuarterCount,
				LastYearCount:    hit.LastYearCount,
			}
		}
	}

	var res []api.TopServicesResponse
	for _, v := range serviceResponse {
		res = append(res, v)
	}
	return res, nil
}

func GetResources(client keibi.Client, rcache *redis.Client, cache *cache.Cache, provider source.Type, sourceID *string, resourceTypes []string) ([]api.ResourceTypeResponse, error) {
	var providerPtr *string
	if provider != "" {
		v := string(provider)
		providerPtr = &v
	}

	var hits []kafka.SourceResourcesSummary
	for _, resourceType := range resourceTypes {
		if cached, err := es.FetchResourceLastSummaryCached(rcache, cache, providerPtr, sourceID, &resourceType); err == nil && len(cached) > 0 {
			hits = append(hits, cached...)
			fmt.Println("fetching resources from cached")
		} else {
			//TODO-Saleh performance issue: use list of resource types instead
			result, err := es.FetchResourceLastSummary(client, providerPtr, sourceID, &resourceType)
			if err != nil {
				return nil, err
			}
			hits = append(hits, result...)
			fmt.Println("fetching resources from ES")
		}
	}

	resourceTypeResponse := map[string]api.ResourceTypeResponse{}
	for _, hit := range hits {
		if v, ok := resourceTypeResponse[hit.ResourceType]; ok {
			v.ResourceCount += hit.ResourceCount
			resourceTypeResponse[hit.ResourceType] = v
		} else {
			resourceTypeResponse[hit.ResourceType] = api.ResourceTypeResponse{
				ResourceType:     cloudservice.ResourceTypeName(hit.ResourceType),
				ResourceCount:    hit.ResourceCount,
				LastDayCount:     hit.LastDayCount,
				LastWeekCount:    hit.LastWeekCount,
				LastQuarterCount: hit.LastQuarterCount,
				LastYearCount:    hit.LastYearCount,
			}
		}
	}

	var res []api.ResourceTypeResponse
	for _, v := range resourceTypeResponse {
		if v.ResourceCount == 0 {
			continue
		}

		res = append(res, v)
	}
	return res, nil
}
