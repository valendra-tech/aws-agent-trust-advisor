package ekstools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
)

// ListEKSClustersTool lists EKS clusters with version and status
type ListEKSClustersTool struct {
	eksClient *eks.Client
}

// NewListEKSClustersTool creates a new instance
func NewListEKSClustersTool(awsConfig aws.Config) *ListEKSClustersTool {
	return &ListEKSClustersTool{
		eksClient: eks.NewFromConfig(awsConfig),
	}
}

// Name returns tool name
func (t *ListEKSClustersTool) Name() string {
	return "list_eks_clusters"
}

// Description describes the tool
func (t *ListEKSClustersTool) Description() string {
	return "Lists EKS clusters including Kubernetes version, status, endpoint, and VPC config summary."
}

// InputSchema returns JSON schema
func (t *ListEKSClustersTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}

// EKSClusterInfo output
type EKSClusterInfo struct {
	Name          string   `json:"name"`
	Status        string   `json:"status"`
	Version       string   `json:"version"`
	Endpoint      *string  `json:"endpoint,omitempty"`
	RoleArn       *string  `json:"role_arn,omitempty"`
	VpcID         *string  `json:"vpc_id,omitempty"`
	SubnetIDs     []string `json:"subnet_ids,omitempty"`
	SecurityGroup *string  `json:"cluster_security_group,omitempty"`
}

// ListEKSClustersOutput response
type ListEKSClustersOutput struct {
	Clusters []EKSClusterInfo `json:"clusters"`
	Count    int              `json:"count"`
}

// Execute lists clusters
func (t *ListEKSClustersTool) Execute(ctx context.Context, input json.RawMessage) (interface{}, error) {
	var names []string
	paginator := eks.NewListClustersPaginator(t.eksClient, &eks.ListClustersInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list EKS clusters: %w", err)
		}
		names = append(names, page.Clusters...)
	}

	if len(names) == 0 {
		return ListEKSClustersOutput{Clusters: []EKSClusterInfo{}, Count: 0}, nil
	}

	var clusters []EKSClusterInfo
	for _, name := range names {
		out, err := t.eksClient.DescribeCluster(ctx, &eks.DescribeClusterInput{
			Name: aws.String(name),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to describe EKS cluster %s: %w", name, err)
		}
		if out.Cluster == nil {
			continue
		}
		c := out.Cluster
		info := EKSClusterInfo{
			Name:     aws.ToString(c.Name),
			Status:   string(c.Status),
			Version:  aws.ToString(c.Version),
			Endpoint: c.Endpoint,
			RoleArn:  c.RoleArn,
		}
		if c.ResourcesVpcConfig != nil {
			if c.ResourcesVpcConfig.VpcId != nil {
				info.VpcID = c.ResourcesVpcConfig.VpcId
			}
			if len(c.ResourcesVpcConfig.SubnetIds) > 0 {
				info.SubnetIDs = append(info.SubnetIDs, c.ResourcesVpcConfig.SubnetIds...)
			}
			if c.ResourcesVpcConfig.ClusterSecurityGroupId != nil {
				info.SecurityGroup = c.ResourcesVpcConfig.ClusterSecurityGroupId
			}
		}
		clusters = append(clusters, info)
	}

	sort.Slice(clusters, func(i, j int) bool { return clusters[i].Name < clusters[j].Name })

	return ListEKSClustersOutput{
		Clusters: clusters,
		Count:    len(clusters),
	}, nil
}
