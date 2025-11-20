package ecstools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
)

// ListServicesTool lists services for a cluster
type ListServicesTool struct {
	ecsClient *ecs.Client
}

// NewListServicesTool creates a new instance
func NewListServicesTool(awsConfig aws.Config) *ListServicesTool {
	return &ListServicesTool{
		ecsClient: ecs.NewFromConfig(awsConfig),
	}
}

// Name returns tool name
func (t *ListServicesTool) Name() string {
	return "list_ecs_services"
}

// Description describes the tool
func (t *ListServicesTool) Description() string {
	return "Lists ECS services for a cluster with desired/running task counts, launch type, and status."
}

// InputSchema returns JSON schema
func (t *ListServicesTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":     "object",
		"required": []string{"cluster"},
		"properties": map[string]interface{}{
			"cluster": map[string]interface{}{
				"type":        "string",
				"description": "ECS cluster name or ARN.",
			},
			"launch_type": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"EC2", "FARGATE"},
				"description": "Optional filter by launch type.",
			},
		},
	}
}

// ListServicesInput parameters
type ListServicesInput struct {
	Cluster    string `json:"cluster"`
	LaunchType string `json:"launch_type,omitempty"`
}

// ServiceInfo output
type ServiceInfo struct {
	Arn            string  `json:"arn"`
	Name           string  `json:"name"`
	Status         string  `json:"status"`
	DesiredCount   int32   `json:"desired_count"`
	RunningCount   int32   `json:"running_count"`
	PendingCount   int32   `json:"pending_count"`
	LaunchType     *string `json:"launch_type,omitempty"`
	SchedulingType *string `json:"scheduling_strategy,omitempty"`
}

// ListServicesOutput response
type ListServicesOutput struct {
	Services []ServiceInfo `json:"services"`
	Count    int           `json:"count"`
}

// Execute lists services
func (t *ListServicesTool) Execute(ctx context.Context, input json.RawMessage) (interface{}, error) {
	var params ListServicesInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("failed to parse input: %w", err)
	}
	if params.Cluster == "" {
		return nil, fmt.Errorf("cluster is required")
	}

	var arns []string
	paginator := ecs.NewListServicesPaginator(t.ecsClient, &ecs.ListServicesInput{
		Cluster: aws.String(params.Cluster),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list ECS services: %w", err)
		}
		arns = append(arns, page.ServiceArns...)
	}

	if len(arns) == 0 {
		return ListServicesOutput{Services: []ServiceInfo{}, Count: 0}, nil
	}

	// Describe services in batches
	var services []ServiceInfo
	for i := 0; i < len(arns); i += 10 {
		end := i + 10
		if end > len(arns) {
			end = len(arns)
		}
		batch := arns[i:end]
		out, err := t.ecsClient.DescribeServices(ctx, &ecs.DescribeServicesInput{
			Cluster:  aws.String(params.Cluster),
			Services: batch,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to describe ECS services: %w", err)
		}
		for _, s := range out.Services {
			lt := aws.ToString((*string)(&s.LaunchType))
			if params.LaunchType != "" && lt != params.LaunchType {
				continue
			}
			svc := ServiceInfo{
				Arn:            aws.ToString(s.ServiceArn),
				Name:           aws.ToString(s.ServiceName),
				Status:         aws.ToString(s.Status),
				DesiredCount:   s.DesiredCount,
				RunningCount:   s.RunningCount,
				PendingCount:   s.PendingCount,
				LaunchType:     nil,
				SchedulingType: nil,
			}
			if s.LaunchType != "" {
				launch := string(s.LaunchType)
				svc.LaunchType = &launch
			}
			if s.SchedulingStrategy != "" {
				sched := string(s.SchedulingStrategy)
				svc.SchedulingType = &sched
			}
			services = append(services, svc)
		}
	}

	sort.Slice(services, func(i, j int) bool { return services[i].Name < services[j].Name })

	return ListServicesOutput{
		Services: services,
		Count:    len(services),
	}, nil
}
