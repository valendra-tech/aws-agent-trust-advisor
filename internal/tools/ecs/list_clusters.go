package ecstools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
)

// ListClustersTool lists ECS clusters with basic metadata
type ListClustersTool struct {
	ecsClient *ecs.Client
}

// NewListClustersTool creates a new instance
func NewListClustersTool(awsConfig aws.Config) *ListClustersTool {
	return &ListClustersTool{
		ecsClient: ecs.NewFromConfig(awsConfig),
	}
}

// Name returns tool name
func (t *ListClustersTool) Name() string {
	return "list_ecs_clusters"
}

// Description describes the tool
func (t *ListClustersTool) Description() string {
	return "Lists ECS clusters with active services/tasks counts, status, and capacity providers."
}

// InputSchema for tool
func (t *ListClustersTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}

// ClusterInfo describes a cluster
type ClusterInfo struct {
	Arn                 string   `json:"arn"`
	Name                string   `json:"name"`
	Status              string   `json:"status"`
	ActiveServicesCount int32    `json:"active_services_count"`
	RunningTasksCount   int32    `json:"running_tasks_count"`
	PendingTasksCount   int32    `json:"pending_tasks_count"`
	CapacityProviders   []string `json:"capacity_providers,omitempty"`
}

// ListClustersOutput response
type ListClustersOutput struct {
	Clusters []ClusterInfo `json:"clusters"`
	Count    int           `json:"count"`
}

// Execute runs the tool
func (t *ListClustersTool) Execute(ctx context.Context, input json.RawMessage) (interface{}, error) {
	// List cluster ARNs
	var arns []string
	paginator := ecs.NewListClustersPaginator(t.ecsClient, &ecs.ListClustersInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list ECS clusters: %w", err)
		}
		arns = append(arns, page.ClusterArns...)
	}

	if len(arns) == 0 {
		return ListClustersOutput{Clusters: []ClusterInfo{}, Count: 0}, nil
	}

	// Describe clusters in batches of 10
	var clusters []ClusterInfo
	for i := 0; i < len(arns); i += 10 {
		end := i + 10
		if end > len(arns) {
			end = len(arns)
		}
		batch := arns[i:end]
		out, err := t.ecsClient.DescribeClusters(ctx, &ecs.DescribeClustersInput{
			Clusters: batch,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to describe ECS clusters: %w", err)
		}
		for _, c := range out.Clusters {
			name := aws.ToString(c.ClusterName)
			status := aws.ToString(c.Status)
			clusters = append(clusters, ClusterInfo{
				Arn:                 aws.ToString(c.ClusterArn),
				Name:                name,
				Status:              status,
				ActiveServicesCount: c.ActiveServicesCount,
				RunningTasksCount:   c.RunningTasksCount,
				PendingTasksCount:   c.PendingTasksCount,
				CapacityProviders:   c.CapacityProviders,
			})
		}
	}

	sort.Slice(clusters, func(i, j int) bool { return clusters[i].Name < clusters[j].Name })

	return ListClustersOutput{
		Clusters: clusters,
		Count:    len(clusters),
	}, nil
}
