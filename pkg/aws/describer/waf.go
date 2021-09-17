package describer

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	"github.com/aws/aws-sdk-go-v2/service/wafregional"
	regionaltypes "github.com/aws/aws-sdk-go-v2/service/wafregional/types"
	"github.com/aws/aws-sdk-go-v2/service/wafv2"
	"github.com/aws/aws-sdk-go-v2/service/wafv2/types"
)

func WAFv2IPSet(ctx context.Context, cfg aws.Config) ([]interface{}, error) {
	client := wafv2.NewFromConfig(cfg)

	scopes := []types.Scope{
		types.ScopeRegional,
	}
	if strings.EqualFold(cfg.Region, "us-east-1") {
		scopes = append(scopes, types.ScopeCloudfront)
	}

	var values []interface{}
	for _, scope := range scopes {
		err := PaginateRetrieveAll(func(prevToken *string) (nextToken *string, err error) {
			output, err := client.ListIPSets(ctx, &wafv2.ListIPSetsInput{
				Scope:      scope,
				NextMarker: prevToken,
			})
			if err != nil {
				return nil, err
			}

			for _, v := range output.IPSets {
				values = append(values, v)
			}
			return output.NextMarker, nil
		})
		if err != nil {
			return nil, err
		}
	}

	return values, nil
}

func WAFv2LoggingConfiguration(ctx context.Context, cfg aws.Config) ([]interface{}, error) {
	client := wafv2.NewFromConfig(cfg)

	scopes := []types.Scope{
		types.ScopeRegional,
	}
	if strings.EqualFold(cfg.Region, "us-east-1") {
		scopes = append(scopes, types.ScopeCloudfront)
	}

	var values []interface{}
	for _, scope := range scopes {
		err := PaginateRetrieveAll(func(prevToken *string) (nextToken *string, err error) {
			output, err := client.ListLoggingConfigurations(ctx, &wafv2.ListLoggingConfigurationsInput{
				Scope:      scope,
				NextMarker: prevToken,
			})
			if err != nil {
				return nil, err
			}

			for _, v := range output.LoggingConfigurations {
				values = append(values, v)
			}
			return output.NextMarker, nil
		})
		if err != nil {
			return nil, err
		}
	}

	return values, nil
}

func WAFv2RegexPatternSet(ctx context.Context, cfg aws.Config) ([]interface{}, error) {
	client := wafv2.NewFromConfig(cfg)

	scopes := []types.Scope{
		types.ScopeRegional,
	}
	if strings.EqualFold(cfg.Region, "us-east-1") {
		scopes = append(scopes, types.ScopeCloudfront)
	}

	var values []interface{}
	for _, scope := range scopes {
		err := PaginateRetrieveAll(func(prevToken *string) (nextToken *string, err error) {
			output, err := client.ListRegexPatternSets(ctx, &wafv2.ListRegexPatternSetsInput{
				Scope:      scope,
				NextMarker: prevToken,
			})
			if err != nil {
				return nil, err
			}

			for _, v := range output.RegexPatternSets {
				values = append(values, v)
			}
			return output.NextMarker, nil
		})
		if err != nil {
			return nil, err
		}
	}

	return values, nil
}

func WAFv2RuleGroup(ctx context.Context, cfg aws.Config) ([]interface{}, error) {
	client := wafv2.NewFromConfig(cfg)

	scopes := []types.Scope{
		types.ScopeRegional,
	}
	if strings.EqualFold(cfg.Region, "us-east-1") {
		scopes = append(scopes, types.ScopeCloudfront)
	}

	var values []interface{}
	for _, scope := range scopes {
		err := PaginateRetrieveAll(func(prevToken *string) (nextToken *string, err error) {
			output, err := client.ListRuleGroups(ctx, &wafv2.ListRuleGroupsInput{
				Scope:      scope,
				NextMarker: prevToken,
			})
			if err != nil {
				return nil, err
			}

			for _, v := range output.RuleGroups {
				values = append(values, v)
			}
			return output.NextMarker, nil
		})
		if err != nil {
			return nil, err
		}
	}

	return values, nil
}

func WAFv2WebACL(ctx context.Context, cfg aws.Config) ([]interface{}, error) {

	scopes := []types.Scope{
		types.ScopeRegional,
	}
	if strings.EqualFold(cfg.Region, "us-east-1") {
		scopes = append(scopes, types.ScopeCloudfront)
	}

	var values []interface{}
	for _, scope := range scopes {
		acls, err := listWAFv2WebACLs(ctx, cfg, scope)
		if err != nil {
			return nil, err
		}

		for _, v := range acls {
			values = append(values, v)
		}
	}

	return values, nil
}

func listWAFv2WebACLs(ctx context.Context, cfg aws.Config, scope types.Scope) ([]types.WebACLSummary, error) {
	client := wafv2.NewFromConfig(cfg)

	var acls []types.WebACLSummary
	err := PaginateRetrieveAll(func(prevToken *string) (nextToken *string, err error) {
		output, err := client.ListWebACLs(ctx, &wafv2.ListWebACLsInput{
			Scope:      scope,
			NextMarker: prevToken,
		})
		if err != nil {
			return nil, err
		}

		acls = append(acls, output.WebACLs...)
		return output.NextMarker, nil
	})
	if err != nil {
		return nil, err
	}

	return acls, nil
}

// Returns ResourceArns that have a WebAcl Associated
func WAFv2WebACLAssociation(ctx context.Context, cfg aws.Config) ([]interface{}, error) {
	var values []interface{}

	regionalACls, err := listWAFv2WebACLs(ctx, cfg, types.ScopeRegional)
	if err != nil {
		return nil, err
	}

	client := wafv2.NewFromConfig(cfg)
	for _, acl := range regionalACls {
		output, err := client.ListResourcesForWebACL(ctx, &wafv2.ListResourcesForWebACLInput{
			WebACLArn: acl.ARN,
		})
		if err != nil {
			return nil, err
		}

		for _, v := range output.ResourceArns {
			values = append(values, v)
		}
	}

	if strings.EqualFold(cfg.Region, "us-east-1") {
		cloudFrontAcls, err := listWAFv2WebACLs(ctx, cfg, types.ScopeCloudfront)
		if err != nil {
			return nil, err
		}

		cfClient := cloudfront.NewFromConfig(cfg)
		for _, acl := range cloudFrontAcls {
			err := PaginateRetrieveAll(func(prevToken *string) (nextToken *string, err error) {
				output, err := cfClient.ListDistributionsByWebACLId(ctx, &cloudfront.ListDistributionsByWebACLIdInput{
					WebACLId: acl.Id,
					Marker:   prevToken,
				})
				if err != nil {
					return nil, err
				}

				for _, v := range output.DistributionList.Items {
					values = append(values, v)
				}

				return output.DistributionList.NextMarker, nil
			})
			if err != nil {
				return nil, err
			}
		}
	}

	return values, nil
}

func WAFRegionalByteMatchSet(ctx context.Context, cfg aws.Config) ([]interface{}, error) {
	client := wafregional.NewFromConfig(cfg)

	var values []interface{}
	err := PaginateRetrieveAll(func(prevToken *string) (nextToken *string, err error) {
		output, err := client.ListByteMatchSets(ctx, &wafregional.ListByteMatchSetsInput{
			NextMarker: prevToken,
		})
		if err != nil {
			return nil, err
		}

		for _, v := range output.ByteMatchSets {
			values = append(values, v)
		}
		return output.NextMarker, nil
	})
	if err != nil {
		return nil, err
	}

	return values, nil
}

func WAFRegionalGeoMatchSet(ctx context.Context, cfg aws.Config) ([]interface{}, error) {
	client := wafregional.NewFromConfig(cfg)

	var values []interface{}
	err := PaginateRetrieveAll(func(prevToken *string) (nextToken *string, err error) {
		output, err := client.ListGeoMatchSets(ctx, &wafregional.ListGeoMatchSetsInput{
			NextMarker: prevToken,
		})
		if err != nil {
			return nil, err
		}

		for _, v := range output.GeoMatchSets {
			values = append(values, v)
		}
		return output.NextMarker, nil
	})
	if err != nil {
		return nil, err
	}

	return values, nil
}

func WAFRegionalIPSet(ctx context.Context, cfg aws.Config) ([]interface{}, error) {
	client := wafregional.NewFromConfig(cfg)

	var values []interface{}
	err := PaginateRetrieveAll(func(prevToken *string) (nextToken *string, err error) {
		output, err := client.ListIPSets(ctx, &wafregional.ListIPSetsInput{
			NextMarker: prevToken,
		})
		if err != nil {
			return nil, err
		}

		for _, v := range output.IPSets {
			values = append(values, v)
		}
		return output.NextMarker, nil
	})
	if err != nil {
		return nil, err
	}

	return values, nil
}

func WAFRegionalRateBasedRule(ctx context.Context, cfg aws.Config) ([]interface{}, error) {
	client := wafregional.NewFromConfig(cfg)

	var values []interface{}
	err := PaginateRetrieveAll(func(prevToken *string) (nextToken *string, err error) {
		output, err := client.ListRateBasedRules(ctx, &wafregional.ListRateBasedRulesInput{
			NextMarker: prevToken,
		})
		if err != nil {
			return nil, err
		}

		for _, v := range output.Rules {
			values = append(values, v)
		}
		return output.NextMarker, nil
	})
	if err != nil {
		return nil, err
	}

	return values, nil
}

func WAFRegionalRegexPatternSet(ctx context.Context, cfg aws.Config) ([]interface{}, error) {
	client := wafregional.NewFromConfig(cfg)

	var values []interface{}
	err := PaginateRetrieveAll(func(prevToken *string) (nextToken *string, err error) {
		output, err := client.ListRegexPatternSets(ctx, &wafregional.ListRegexPatternSetsInput{
			NextMarker: prevToken,
		})
		if err != nil {
			return nil, err
		}

		for _, v := range output.RegexPatternSets {
			values = append(values, v)
		}
		return output.NextMarker, nil
	})
	if err != nil {
		return nil, err
	}

	return values, nil
}

func WAFRegionalRule(ctx context.Context, cfg aws.Config) ([]interface{}, error) {
	client := wafregional.NewFromConfig(cfg)

	var values []interface{}
	err := PaginateRetrieveAll(func(prevToken *string) (nextToken *string, err error) {
		output, err := client.ListRules(ctx, &wafregional.ListRulesInput{
			NextMarker: prevToken,
		})
		if err != nil {
			return nil, err
		}

		for _, v := range output.Rules {
			values = append(values, v)
		}
		return output.NextMarker, nil
	})
	if err != nil {
		return nil, err
	}

	return values, nil
}

func WAFRegionalSizeConstraintSet(ctx context.Context, cfg aws.Config) ([]interface{}, error) {
	client := wafregional.NewFromConfig(cfg)

	var values []interface{}
	err := PaginateRetrieveAll(func(prevToken *string) (nextToken *string, err error) {
		output, err := client.ListSizeConstraintSets(ctx, &wafregional.ListSizeConstraintSetsInput{
			NextMarker: prevToken,
		})
		if err != nil {
			return nil, err
		}

		for _, v := range output.SizeConstraintSets {
			values = append(values, v)
		}
		return output.NextMarker, nil
	})
	if err != nil {
		return nil, err
	}

	return values, nil
}

func WAFRegionalSqlInjectionMatchSet(ctx context.Context, cfg aws.Config) ([]interface{}, error) {
	client := wafregional.NewFromConfig(cfg)

	var values []interface{}
	err := PaginateRetrieveAll(func(prevToken *string) (nextToken *string, err error) {
		output, err := client.ListSqlInjectionMatchSets(ctx, &wafregional.ListSqlInjectionMatchSetsInput{
			NextMarker: prevToken,
		})
		if err != nil {
			return nil, err
		}

		for _, v := range output.SqlInjectionMatchSets {
			values = append(values, v)
		}
		return output.NextMarker, nil
	})
	if err != nil {
		return nil, err
	}

	return values, nil
}

func WAFRegionalWebACL(ctx context.Context, cfg aws.Config) ([]interface{}, error) {
	client := wafregional.NewFromConfig(cfg)

	var values []interface{}
	err := PaginateRetrieveAll(func(prevToken *string) (nextToken *string, err error) {
		output, err := client.ListWebACLs(ctx, &wafregional.ListWebACLsInput{
			NextMarker: prevToken,
		})
		if err != nil {
			return nil, err
		}

		for _, v := range output.WebACLs {
			values = append(values, v)
		}
		return output.NextMarker, nil
	})
	if err != nil {
		return nil, err
	}

	return values, nil
}

func WAFRegionalWebACLAssociation(ctx context.Context, cfg aws.Config) ([]interface{}, error) {
	acls, err := WAFRegionalWebACL(ctx, cfg)
	if err != nil {
		return nil, err
	}

	client := wafregional.NewFromConfig(cfg)

	var values []interface{}
	for _, a := range acls {
		acl := a.(regionaltypes.WebACLSummary)
		output, err := client.ListResourcesForWebACL(ctx, &wafregional.ListResourcesForWebACLInput{
			WebACLId: acl.WebACLId,
		})
		if err != nil {
			return nil, err
		}

		for _, v := range output.ResourceArns {
			values = append(values, v)
		}
	}

	return values, nil
}

func WAFRegionalXssMatchSet(ctx context.Context, cfg aws.Config) ([]interface{}, error) {
	client := wafregional.NewFromConfig(cfg)

	var values []interface{}
	err := PaginateRetrieveAll(func(prevToken *string) (nextToken *string, err error) {
		output, err := client.ListXssMatchSets(ctx, &wafregional.ListXssMatchSetsInput{
			NextMarker: prevToken,
		})
		if err != nil {
			return nil, err
		}

		for _, v := range output.XssMatchSets {
			values = append(values, v)
		}
		return output.NextMarker, nil
	})
	if err != nil {
		return nil, err
	}

	return values, nil
}
