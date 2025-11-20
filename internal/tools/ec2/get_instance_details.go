package ec2tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// GetInstanceDetailsTool returns rich information about a single EC2 instance
type GetInstanceDetailsTool struct {
	ec2Client *ec2.Client
}

// NewGetInstanceDetailsTool creates a new instance
func NewGetInstanceDetailsTool(awsConfig aws.Config) *GetInstanceDetailsTool {
	return &GetInstanceDetailsTool{
		ec2Client: ec2.NewFromConfig(awsConfig),
	}
}

// Name returns the tool name
func (t *GetInstanceDetailsTool) Name() string {
	return "get_ec2_instance_details"
}

// Description describes the tool
func (t *GetInstanceDetailsTool) Description() string {
	return "Gets detailed information for a specific EC2 instance: networking, storage, platform, CPU options, IAM profile, and tags."
}

// InputSchema returns the JSON schema
func (t *GetInstanceDetailsTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"required":   []string{"instance_id"},
		"properties": map[string]interface{}{"instance_id": map[string]interface{}{"type": "string", "description": "The ID of the EC2 instance (i-xxxxxxxxxx)."}},
	}
}

// GetInstanceDetailsInput holds parameters
type GetInstanceDetailsInput struct {
	InstanceID string `json:"instance_id"`
}

// NetworkInterfaceInfo describes ENIs
type NetworkInterfaceInfo struct {
	NetworkInterfaceID string             `json:"network_interface_id"`
	PrivateIP          *string            `json:"private_ip,omitempty"`
	PublicIP           *string            `json:"public_ip,omitempty"`
	SubnetID           *string            `json:"subnet_id,omitempty"`
	VpcID              *string            `json:"vpc_id,omitempty"`
	SecurityGroups     []SecurityGroupRef `json:"security_groups,omitempty"`
}

// VolumeAttachmentInfo describes attached volumes
type VolumeAttachmentInfo struct {
	VolumeID   string  `json:"volume_id"`
	DeviceName *string `json:"device_name,omitempty"`
	State      *string `json:"state,omitempty"`
}

// InstanceDetailsResponse carries data
type InstanceDetailsResponse struct {
	InstanceID         string                 `json:"instance_id"`
	State              *string                `json:"state,omitempty"`
	InstanceType       *string                `json:"instance_type,omitempty"`
	AvailabilityZone   *string                `json:"availability_zone,omitempty"`
	ImageID            *string                `json:"image_id,omitempty"`
	Platform           *string                `json:"platform,omitempty"`
	Architecture       *string                `json:"architecture,omitempty"`
	LaunchTime         *string                `json:"launch_time,omitempty"`
	IamInstanceProfile *string                `json:"iam_instance_profile,omitempty"`
	CPUOptions         *types.CpuOptions      `json:"cpu_options,omitempty"`
	Hibernation        *bool                  `json:"hibernation_enabled,omitempty"`
	Monitoring         *string                `json:"monitoring,omitempty"`
	SecurityGroups     []SecurityGroupRef     `json:"security_groups,omitempty"`
	Network            []NetworkInterfaceInfo `json:"network_interfaces,omitempty"`
	Volumes            []VolumeAttachmentInfo `json:"volumes,omitempty"`
	Tags               map[string]string      `json:"tags,omitempty"`
}

// Execute fetches details
func (t *GetInstanceDetailsTool) Execute(ctx context.Context, input json.RawMessage) (interface{}, error) {
	var params GetInstanceDetailsInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("failed to parse input: %w", err)
	}
	if params.InstanceID == "" {
		return nil, fmt.Errorf("instance_id is required")
	}

	out, err := t.ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{params.InstanceID},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe instance %s: %w", params.InstanceID, err)
	}
	if len(out.Reservations) == 0 || len(out.Reservations[0].Instances) == 0 {
		return nil, fmt.Errorf("instance %s not found", params.InstanceID)
	}
	inst := out.Reservations[0].Instances[0]

	resp := InstanceDetailsResponse{
		InstanceID: params.InstanceID,
	}

	if inst.State != nil && inst.State.Name != "" {
		state := string(inst.State.Name)
		resp.State = &state
	}
	if inst.InstanceType != "" {
		tp := string(inst.InstanceType)
		resp.InstanceType = &tp
	}
	if inst.Placement != nil && inst.Placement.AvailabilityZone != nil {
		resp.AvailabilityZone = inst.Placement.AvailabilityZone
	}
	if inst.ImageId != nil {
		resp.ImageID = inst.ImageId
	}
	if inst.Architecture != "" {
		arch := string(inst.Architecture)
		resp.Architecture = &arch
	}
	if inst.Platform != "" {
		pl := string(inst.Platform)
		resp.Platform = &pl
	}
	if inst.LaunchTime != nil {
		lt := inst.LaunchTime.In(time.UTC).Format(time.RFC3339)
		resp.LaunchTime = &lt
	}
	if inst.IamInstanceProfile != nil && inst.IamInstanceProfile.Arn != nil {
		resp.IamInstanceProfile = inst.IamInstanceProfile.Arn
	}
	if inst.CpuOptions != nil {
		resp.CPUOptions = inst.CpuOptions
	}
	if inst.HibernationOptions != nil {
		resp.Hibernation = inst.HibernationOptions.Configured
	}
	if inst.Monitoring != nil {
		m := string(inst.Monitoring.State)
		resp.Monitoring = &m
	}

	for _, eni := range inst.NetworkInterfaces {
		if eni.NetworkInterfaceId == nil {
			continue
		}
		info := NetworkInterfaceInfo{
			NetworkInterfaceID: *eni.NetworkInterfaceId,
			PrivateIP:          eni.PrivateIpAddress,
			VpcID:              eni.VpcId,
			SubnetID:           eni.SubnetId,
		}
		if eni.Association != nil && eni.Association.PublicIp != nil {
			info.PublicIP = eni.Association.PublicIp
		}
		for _, sg := range eni.Groups {
			if sg.GroupId != nil {
				ref := SecurityGroupRef{
					GroupID:   *sg.GroupId,
					GroupName: sg.GroupName,
				}
				info.SecurityGroups = append(info.SecurityGroups, ref)
			}
		}
		resp.Network = append(resp.Network, info)
	}

	// Aggregate unique SGs at instance level
	seenSG := make(map[string]bool)
	for _, sg := range inst.SecurityGroups {
		if sg.GroupId == nil {
			continue
		}
		id := *sg.GroupId
		if seenSG[id] {
			continue
		}
		seenSG[id] = true
		resp.SecurityGroups = append(resp.SecurityGroups, SecurityGroupRef{
			GroupID:   id,
			GroupName: sg.GroupName,
		})
	}

	for _, mapping := range inst.BlockDeviceMappings {
		if mapping.Ebs == nil || mapping.Ebs.VolumeId == nil {
			continue
		}
		info := VolumeAttachmentInfo{
			VolumeID: *mapping.Ebs.VolumeId,
		}
		if mapping.DeviceName != nil {
			info.DeviceName = mapping.DeviceName
		}
		if mapping.Ebs.Status != "" {
			st := string(mapping.Ebs.Status)
			info.State = &st
		}
		resp.Volumes = append(resp.Volumes, info)
	}

	if len(inst.Tags) > 0 {
		resp.Tags = make(map[string]string, len(inst.Tags))
		for _, tag := range inst.Tags {
			if tag.Key != nil && tag.Value != nil {
				resp.Tags[*tag.Key] = *tag.Value
			}
		}
	}

	return resp, nil
}
