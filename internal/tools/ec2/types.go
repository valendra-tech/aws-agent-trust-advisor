package ec2tools

// SecurityGroupRef represents a security group reference
type SecurityGroupRef struct {
	GroupID   string  `json:"group_id"`
	GroupName *string `json:"group_name,omitempty"`
}
