package container

import (
	"context"
	"fmt"

	"go.uber.org/dig"

	"github.com/valendra-tech/aws-agent-trust-advisor/internal/services/aws"
	"github.com/valendra-tech/aws-agent-trust-advisor/internal/services/bedrock"
	"github.com/valendra-tech/aws-agent-trust-advisor/internal/services/logger"
	"github.com/valendra-tech/aws-agent-trust-advisor/internal/tools"
	cloudtrailtools "github.com/valendra-tech/aws-agent-trust-advisor/internal/tools/cloudtrail"
	cloudwatchtools "github.com/valendra-tech/aws-agent-trust-advisor/internal/tools/cloudwatch"
	cloudwatchlogstools "github.com/valendra-tech/aws-agent-trust-advisor/internal/tools/cloudwatchlogs"
	costexplorertools "github.com/valendra-tech/aws-agent-trust-advisor/internal/tools/costexplorer"
	ec2tools "github.com/valendra-tech/aws-agent-trust-advisor/internal/tools/ec2"
	ecstools "github.com/valendra-tech/aws-agent-trust-advisor/internal/tools/ecs"
	ekstools "github.com/valendra-tech/aws-agent-trust-advisor/internal/tools/eks"
	elbtools "github.com/valendra-tech/aws-agent-trust-advisor/internal/tools/elb"
	s3tools "github.com/valendra-tech/aws-agent-trust-advisor/internal/tools/s3"
	organizationstools "github.com/valendra-tech/aws-agent-trust-advisor/internal/tools/organizations"
	sestools "github.com/valendra-tech/aws-agent-trust-advisor/internal/tools/ses"
	vpctools "github.com/valendra-tech/aws-agent-trust-advisor/internal/tools/vpc"
)

type Container struct {
	*dig.Container
}

func New() *Container {
	return &Container{
		Container: dig.New(),
	}
}

type BuildParams struct {
	AWSProfile string
	AWSRegion  string
	LogLevel   string
}

func (c *Container) Build(params BuildParams) error {
	if err := c.Provide(func() *logger.Logger {
		return logger.New(params.LogLevel)
	}); err != nil {
		return fmt.Errorf("failed to provide logger: %w", err)
	}

	if err := c.Provide(func(log *logger.Logger) (*aws.Service, error) {
		return aws.New(context.Background(), aws.Config{
			Profile: params.AWSProfile,
			Region:  params.AWSRegion,
		}, log)
	}); err != nil {
		return fmt.Errorf("failed to provide AWS service: %w", err)
	}

	if err := c.Provide(func(log *logger.Logger, awsSvc *aws.Service) *bedrock.Service {
		return bedrock.New(awsSvc.GetConfig(), log)
	}); err != nil {
		return fmt.Errorf("failed to provide Bedrock service: %w", err)
	}

	// Register tools registry
	if err := c.Provide(func(log *logger.Logger, awsSvc *aws.Service) *tools.Registry {
		registry := tools.NewRegistry()
		awsConfig := awsSvc.GetConfig()

		// Register S3 tools
		listBucketsTool := s3tools.NewListBucketsTool(awsConfig)
		if err := registry.Register(listBucketsTool); err != nil {
			log.Error("Failed to register list_buckets tool: %v", err)
		}

		// Register EC2 tools
		listInstancesTool := ec2tools.NewListInstancesTool(awsConfig)
		if err := registry.Register(listInstancesTool); err != nil {
			log.Error("Failed to register list_ec2_instances tool: %v", err)
		}
		getInstanceDetailsTool := ec2tools.NewGetInstanceDetailsTool(awsConfig)
		if err := registry.Register(getInstanceDetailsTool); err != nil {
			log.Error("Failed to register get_ec2_instance_details tool: %v", err)
		}
		listSecurityGroupsTool := ec2tools.NewListSecurityGroupsTool(awsConfig)
		if err := registry.Register(listSecurityGroupsTool); err != nil {
			log.Error("Failed to register list_security_groups tool: %v", err)
		}
		listVolumesTool := ec2tools.NewListVolumesTool(awsConfig)
		if err := registry.Register(listVolumesTool); err != nil {
			log.Error("Failed to register list_ec2_volumes tool: %v", err)
		}

		// Register CloudWatch tools
		listNamespacesTool := cloudwatchtools.NewListNamespacesTool(awsConfig)
		if err := registry.Register(listNamespacesTool); err != nil {
			log.Error("Failed to register list_cloudwatch_namespaces tool: %v", err)
		}

		listMetricsTool := cloudwatchtools.NewListMetricsTool(awsConfig)
		if err := registry.Register(listMetricsTool); err != nil {
			log.Error("Failed to register list_cloudwatch_metrics tool: %v", err)
		}

		getMetricStatisticsTool := cloudwatchtools.NewGetMetricStatisticsTool(awsConfig)
		if err := registry.Register(getMetricStatisticsTool); err != nil {
			log.Error("Failed to register get_cloudwatch_metric_statistics tool: %v", err)
		}

		// Register CloudWatch Logs tools
		listLogGroupsTool := cloudwatchlogstools.NewListLogGroupsTool(awsConfig)
		if err := registry.Register(listLogGroupsTool); err != nil {
			log.Error("Failed to register list_log_groups tool: %v", err)
		}
		getLogEventsTool := cloudwatchlogstools.NewGetLogEventsTool(awsConfig)
		if err := registry.Register(getLogEventsTool); err != nil {
			log.Error("Failed to register get_log_events tool: %v", err)
		}

		// Register ECS tools
		listECSClustersTool := ecstools.NewListClustersTool(awsConfig)
		if err := registry.Register(listECSClustersTool); err != nil {
			log.Error("Failed to register list_ecs_clusters tool: %v", err)
		}
		listECSServicesTool := ecstools.NewListServicesTool(awsConfig)
		if err := registry.Register(listECSServicesTool); err != nil {
			log.Error("Failed to register list_ecs_services tool: %v", err)
		}

		// Register EKS tools
		listEKSClustersTool := ekstools.NewListEKSClustersTool(awsConfig)
		if err := registry.Register(listEKSClustersTool); err != nil {
			log.Error("Failed to register list_eks_clusters tool: %v", err)
		}

		// Register ELB tools
		listLoadBalancersTool := elbtools.NewListLoadBalancersTool(awsConfig)
		if err := registry.Register(listLoadBalancersTool); err != nil {
			log.Error("Failed to register list_load_balancers tool: %v", err)
		}

		listTargetGroupsTool := elbtools.NewListTargetGroupsTool(awsConfig)
		if err := registry.Register(listTargetGroupsTool); err != nil {
			log.Error("Failed to register list_target_groups tool: %v", err)
		}

		// Register Cost Explorer tools
		getCostAndUsageTool := costexplorertools.NewGetCostAndUsageTool(awsConfig)
		if err := registry.Register(getCostAndUsageTool); err != nil {
			log.Error("Failed to register get_cost_and_usage tool: %v", err)
		}

		getCostForecastTool := costexplorertools.NewGetCostForecastTool(awsConfig)
		if err := registry.Register(getCostForecastTool); err != nil {
			log.Error("Failed to register get_cost_forecast tool: %v", err)
		}

		getTopCostsTool := costexplorertools.NewGetTopCostsTool(awsConfig)
		if err := registry.Register(getTopCostsTool); err != nil {
			log.Error("Failed to register get_top_costs tool: %v", err)
		}

		// Register CloudTrail tools
		listTrailsTool := cloudtrailtools.NewListTrailsTool(awsConfig)
		if err := registry.Register(listTrailsTool); err != nil {
			log.Error("Failed to register list_cloudtrail_trails tool: %v", err)
		}
		lookupEventsTool := cloudtrailtools.NewLookupEventsTool(awsConfig)
		if err := registry.Register(lookupEventsTool); err != nil {
			log.Error("Failed to register lookup_cloudtrail_events tool: %v", err)
		}

		// Register VPC tools
		listVPCsTool := vpctools.NewListVPCsTool(awsConfig)
		if err := registry.Register(listVPCsTool); err != nil {
			log.Error("Failed to register list_vpcs tool: %v", err)
		}
		listNATTool := vpctools.NewListNATGatewaysTool(awsConfig)
		if err := registry.Register(listNATTool); err != nil {
			log.Error("Failed to register list_nat_gateways tool: %v", err)
		}
		listRouteTablesTool := vpctools.NewListRouteTablesTool(awsConfig)
		if err := registry.Register(listRouteTablesTool); err != nil {
			log.Error("Failed to register list_route_tables tool: %v", err)
		}
		listPeeringTool := vpctools.NewListPeeringConnectionsTool(awsConfig)
		if err := registry.Register(listPeeringTool); err != nil {
			log.Error("Failed to register list_vpc_peering_connections tool: %v", err)
		}
		listFlowLogsTool := vpctools.NewListFlowLogsTool(awsConfig)
		if err := registry.Register(listFlowLogsTool); err != nil {
			log.Error("Failed to register list_flow_logs tool: %v", err)
		}

		// Register SES tools
		sesAccountTool := sestools.NewAccountOverviewTool(awsConfig)
		if err := registry.Register(sesAccountTool); err != nil {
			log.Error("Failed to register get_ses_account_overview tool: %v", err)
		}
		listIdentitiesTool := sestools.NewListEmailIdentitiesTool(awsConfig)
		if err := registry.Register(listIdentitiesTool); err != nil {
			log.Error("Failed to register list_ses_email_identities tool: %v", err)
		}
		listConfigSetsTool := sestools.NewListConfigurationSetsTool(awsConfig)
		if err := registry.Register(listConfigSetsTool); err != nil {
			log.Error("Failed to register list_ses_configuration_sets tool: %v", err)
		}

		// Register Organizations tools
		describeOrgTool := organizationstools.NewDescribeOrganizationTool(awsConfig)
		if err := registry.Register(describeOrgTool); err != nil {
			log.Error("Failed to register describe_organization tool: %v", err)
		}
		listAccountsTool := organizationstools.NewListAccountsTool(awsConfig)
		if err := registry.Register(listAccountsTool); err != nil {
			log.Error("Failed to register list_organization_accounts tool: %v", err)
		}
		listPoliciesTool := organizationstools.NewListPoliciesTool(awsConfig)
		if err := registry.Register(listPoliciesTool); err != nil {
			log.Error("Failed to register list_organization_policies tool: %v", err)
		}

		log.Info("Registered %d tools", registry.Count())
		return registry
	}); err != nil {
		return fmt.Errorf("failed to provide tools registry: %w", err)
	}

	return nil
}
