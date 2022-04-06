module gitlab.com/keibiengine/keibi-engine

go 1.16

require (
	github.com/Azure/azure-sdk-for-go v59.3.0+incompatible
	github.com/Azure/go-autorest/autorest v0.11.22
	github.com/Azure/go-autorest/autorest/azure/auth v0.5.9
	github.com/aws/aws-sdk-go v1.42.48
	github.com/aws/aws-sdk-go-v2 v1.13.0
	github.com/aws/aws-sdk-go-v2/config v1.10.2
	github.com/aws/aws-sdk-go-v2/credentials v1.6.2
	github.com/aws/aws-sdk-go-v2/service/accessanalyzer v1.12.0
	github.com/aws/aws-sdk-go-v2/service/acm v1.10.0
	github.com/aws/aws-sdk-go-v2/service/apigateway v1.11.0
	github.com/aws/aws-sdk-go-v2/service/apigatewayv2 v1.10.0
	github.com/aws/aws-sdk-go-v2/service/applicationautoscaling v1.11.0
	github.com/aws/aws-sdk-go-v2/service/applicationinsights v1.8.0
	github.com/aws/aws-sdk-go-v2/service/autoscaling v1.17.0
	github.com/aws/aws-sdk-go-v2/service/backup v1.9.1
	github.com/aws/aws-sdk-go-v2/service/cloudfront v1.11.1
	github.com/aws/aws-sdk-go-v2/service/cloudtrail v1.11.0
	github.com/aws/aws-sdk-go-v2/service/cloudwatch v1.12.0
	github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs v1.10.1
	github.com/aws/aws-sdk-go-v2/service/codebuild v1.15.0
	github.com/aws/aws-sdk-go-v2/service/configservice v1.13.0
	github.com/aws/aws-sdk-go-v2/service/databasemigrationservice v1.14.0
	github.com/aws/aws-sdk-go-v2/service/dax v1.7.2
	github.com/aws/aws-sdk-go-v2/service/dynamodb v1.11.0
	github.com/aws/aws-sdk-go-v2/service/ec2 v1.26.0
	github.com/aws/aws-sdk-go-v2/service/ecr v1.10.1
	github.com/aws/aws-sdk-go-v2/service/ecrpublic v1.8.1
	github.com/aws/aws-sdk-go-v2/service/ecs v1.12.1
	github.com/aws/aws-sdk-go-v2/service/efs v1.10.1
	github.com/aws/aws-sdk-go-v2/service/eks v1.14.0
	github.com/aws/aws-sdk-go-v2/service/elasticache v1.16.0
	github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk v1.10.0
	github.com/aws/aws-sdk-go-v2/service/elasticloadbalancing v1.9.1
	github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2 v1.12.1
	github.com/aws/aws-sdk-go-v2/service/elasticsearchservice v1.11.0
	github.com/aws/aws-sdk-go-v2/service/emr v1.14.0
	github.com/aws/aws-sdk-go-v2/service/fsx v1.17.0
	github.com/aws/aws-sdk-go-v2/service/guardduty v1.9.0
	github.com/aws/aws-sdk-go-v2/service/iam v1.15.0
	github.com/aws/aws-sdk-go-v2/service/kms v1.11.0
	github.com/aws/aws-sdk-go-v2/service/lambda v1.13.0
	github.com/aws/aws-sdk-go-v2/service/organizations v1.10.0
	github.com/aws/aws-sdk-go-v2/service/rds v1.12.1
	github.com/aws/aws-sdk-go-v2/service/redshift v1.15.0
	github.com/aws/aws-sdk-go-v2/service/route53 v1.14.1
	github.com/aws/aws-sdk-go-v2/service/route53resolver v1.10.1
	github.com/aws/aws-sdk-go-v2/service/s3 v1.22.0
	github.com/aws/aws-sdk-go-v2/service/s3control v1.17.0
	github.com/aws/aws-sdk-go-v2/service/sagemaker v1.21.0
	github.com/aws/aws-sdk-go-v2/service/secretsmanager v1.11.0
	github.com/aws/aws-sdk-go-v2/service/securityhub v1.16.0
	github.com/aws/aws-sdk-go-v2/service/ses v1.9.1
	github.com/aws/aws-sdk-go-v2/service/sesv2 v1.8.1
	github.com/aws/aws-sdk-go-v2/service/sns v1.12.0
	github.com/aws/aws-sdk-go-v2/service/sqs v1.12.1
	github.com/aws/aws-sdk-go-v2/service/ssm v1.16.0
	github.com/aws/aws-sdk-go-v2/service/sts v1.10.1
	github.com/aws/aws-sdk-go-v2/service/support v1.11.0
	github.com/aws/aws-sdk-go-v2/service/synthetics v1.9.1
	github.com/aws/aws-sdk-go-v2/service/wafregional v1.8.1
	github.com/aws/aws-sdk-go-v2/service/wafv2 v1.14.0
	github.com/aws/aws-sdk-go-v2/service/workspaces v1.10.1
	github.com/aws/smithy-go v1.10.0
	github.com/cenkalti/backoff/v3 v3.2.2
	github.com/elastic/go-elasticsearch/v7 v7.16.0
	github.com/gocarina/gocsv v0.0.0-20211203214250-4735fba0c1d9
	github.com/gofrs/uuid v4.0.0+incompatible
	github.com/golang/protobuf v1.5.2
	github.com/google/uuid v1.3.0
	github.com/hashicorp/go-hclog v1.0.0
	github.com/hashicorp/vault/api v1.3.0
	github.com/hashicorp/vault/api/auth/kubernetes v0.1.0
	github.com/jackc/pgx/v4 v4.13.0
	github.com/labstack/echo/v4 v4.6.1
	github.com/labstack/gommon v0.3.1
	github.com/manicminer/hamilton v0.41.1
	github.com/ory/dockertest/v3 v3.8.1
	github.com/spf13/cobra v1.3.0
	github.com/streadway/amqp v1.0.0
	github.com/stretchr/testify v1.7.0
	github.com/swaggo/echo-swagger v1.3.0
	github.com/swaggo/swag v1.8.0
	github.com/tombuildsstuff/giovanni v0.18.0
	github.com/turbot/go-kit v0.3.0
	github.com/turbot/steampipe-plugin-sdk v1.8.3
	gitlab.com/keibiengine/steampipe-plugin-azure v0.23.2-0.20220401174801-d61359c2d790
	gitlab.com/keibiengine/steampipe-plugin-azuread v0.1.1-0.20220401174905-60626bc2deea
	gitlab.com/keibiengine/steampipe-plugin-aws v0.0.0-20220401174834-4d29274e8abe
	go.uber.org/zap v1.21.0
	gopkg.in/Shopify/sarama.v1 v1.20.1
	gopkg.in/go-playground/validator.v9 v9.31.0
	gorm.io/driver/postgres v1.2.2
	gorm.io/gorm v1.22.3
)
