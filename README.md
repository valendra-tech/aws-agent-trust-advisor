# AWS Agent Trust Advisor

## Privacy
- All AWS data stays within your AWS account; the CLI invokes AWS APIs directly with your credentials.
- Model calls go to **Amazon Bedrock**; prompts and tool results are sent there for reasoning. Bedrock is fully managed within AWS regions and doesn’t export your data outside AWS.
- Tools are read-only; no mutations are performed.

AI-assisted CLI to inspect and troubleshoot your AWS environment directly from the terminal. It chats through Bedrock models and invokes a broad catalog of read-only AWS tools—no extra setup beyond your standard AWS credentials.

## Highlights
- **Interactive by default**: `./aws-agent-trust-advisor` opens a chat loop with tool use.
- **One-shot mode**: `--prompt "text"` runs a single request and exits.
- **No extra credential config**: uses the AWS SDK v2 default chain (env vars, shared config/credentials, SSO, IMDS). If `aws sts get-caller-identity` works, you’re ready.
- **Tools-first answers**: responses are grounded in live AWS API calls; tool errors are surfaced both to you and to the model.
- **Single static binary**: Go 1.25.4+, no services to deploy.

## Quickstart
```bash
# easiest: install script (sudo because it writes to /usr/local/bin)
curl -fsSL https://raw.githubusercontent.com/valendra-tech/aws-agent-trust-advisor/main/install.sh | sudo bash

# or download a release asset yourself
# e.g., curl -L -o aws-agent-trust-advisor <release-asset-url> && chmod +x aws-agent-trust-advisor

# build
go build -o aws-agent-trust-advisor

# interactive (default region eu-west-1)
./aws-agent-trust-advisor

# choose profile/region
./aws-agent-trust-advisor -p myprofile -r us-east-1

# one-shot prompt
./aws-agent-trust-advisor --prompt "List my S3 buckets" -p myprofile
```

### Key flags
- `--profile, -p` AWS profile  
- `--region, -r` AWS region  
- `--prompt, -P` single-prompt mode (disables interactive loop)  
- `--interactive, -i` default **true**; set `--interactive=false` when using only `--prompt`  
- `--model, -m` Bedrock model ID (default `openai.gpt-oss-120b-1:0`)  
- `--system-prompt, -s` custom system prompt file  

## Tool Catalog
| Tool | Purpose | Important inputs |
| --- | --- | --- |
| list_s3_buckets | Buckets with region/versioning/encryption/public/object count | fields, limit, offset |
| list_cloudwatch_namespaces | CloudWatch namespaces | – |
| list_cloudwatch_metrics | Metrics by namespace | namespace, metric_name, dimensions |
| get_cloudwatch_metric_statistics | Metric stats | namespace, metric_name, dimensions, start_time, end_time, period |
| list_log_groups | CloudWatch log groups | prefix, limit, offset |
| get_log_events | Log events (filter or stream) | log_group, log_stream?, filter_pattern?, start_time_ms, end_time_ms, limit |
| list_ec2_instances | EC2 instances (AZ, IPs, SGs, tags) | states, tag_key/value, include_tags, limit/offset |
| get_ec2_instance_details | Detailed EC2 info (ENIs, vols, SGs, tags) | instance_id |
| list_security_groups | SGs with rule counts and tags | vpc_id, tag_key/value, include_tags, limit/offset |
| list_ec2_volumes | EBS volumes | state, availability_zone, tag_key/value, limit/offset |
| list_ecs_clusters | ECS clusters | – |
| list_ecs_services | ECS services per cluster | cluster, launch_type |
| list_eks_clusters | EKS clusters | – |
| list_load_balancers | ALB/NLB/GLB | load_balancer_type, fields, limit/offset |
| list_target_groups | Target groups | limit/offset |
| get_cost_and_usage | Cost Explorer usage | start, end, granularity, metrics, group_by |
| get_cost_forecast | Cost forecast | start, end, metric, granularity |
| get_top_costs | Top-N costs by dimension | dimension, start/end, granularity, top_n |
| list_cloudtrail_trails | CloudTrail trails/status | – |
| lookup_cloudtrail_events | CloudTrail events | event_name, username, resource_name/type, start_time, end_time, limit |
| list_vpcs | VPCs (CIDR, DNS flags, tags) | tag_key/value, include_tags |
| list_nat_gateways | NAT gateways | vpc_id, subnet_id |
| list_route_tables | Route tables | vpc_id |
| list_vpc_peering_connections | VPC peerings | status |
| list_flow_logs | VPC flow logs | resource_id, limit |
| list_rds_instances | RDS instances (SGs, optional tags) | fields, include_tags, limit/offset |
| get_rds_instance_details | RDS details | identifier |
| list_ses_email_identities | SES identities | verification_status, identity_type, limit/offset |
| get_ses_account_overview | SES quotas & production access | – |
| list_ses_configuration_sets | SES configuration sets | limit/offset |

## How it works
1. DI container (Uber dig) wires AWS SDK, Bedrock client, logger, and all tools.  
2. The model proposes `tool_calls`; the CLI executes them and returns JSON results.  
3. If a tool fails, the CLI sends `error: <detail>` back to the model **and** prints it locally, so loops don’t get stuck.

## Requirements
- Go 1.25.4+
- AWS credentials reachable via the standard AWS SDK chain (env/shared config/SSO/IMDS).
- Read permissions for the services you want to query (STS, EC2, S3, CloudWatch, etc.).

## Development
```bash
go test ./...                 # uses local GOCACHE if global cache is blocked
gofmt -w .                    # formatting
```

## License
GPLv3 (see LICENSE).
