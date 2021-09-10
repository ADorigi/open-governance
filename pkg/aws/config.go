package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// getConfig loads the AWS credentionals and returns the configuration to be used by the AWS services client.
// If the awsAccessKey is specified, the config will be created for the combination of awsAccessKey, awsSecretKey, awsSessionToken.
// Else it will use the default AWS SDK logic to load the configuration. See https://aws.github.io/aws-sdk-go-v2/docs/configuring-sdk/
// If assumeRoleArn is provided, it will use the evaluated configuration to then assume the specified role.
func getConfig(ctx context.Context, awsAccessKey, awsSecretKey, awsSessionToken, assumeRoleArn string) (aws.Config, error) {
	opts := []func(*config.LoadOptions) error{}

	if awsAccessKey != "" {
		opts = append(opts, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(awsAccessKey, awsSecretKey, awsSessionToken)))
	}

	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to load AWS config: %w", err)
	}

	if assumeRoleArn != "" {
		cfg, err = config.LoadDefaultConfig(context.Background(), config.WithCredentialsProvider(stscreds.NewAssumeRoleProvider(sts.NewFromConfig(cfg), assumeRoleArn)))
		if err != nil {
			return aws.Config{}, fmt.Errorf("failed to assume role: %w", err)
		}
	}

	return cfg, nil
}
