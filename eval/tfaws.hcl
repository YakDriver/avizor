# Radar evaluation corpus for the Terraform AWS Provider.
#
# Every expectation comes from repository history, not from Radar's output:
#   duplicate          GitHub "Duplicate of #N" markings. Every 5th of the 98 pairs filed since 2025-01-01 whose original
#                      predates the duplicate, in issue order.
#   announcement       What's New URLs linked from maintainer tracking issues, reconciled 30 minutes after the tracker was
#                      filed.
#   announcement-text  The same announcements' headlines as free text, as a differently worded request would arrive.
#   implemented        Announcements reconciled an hour after the PR that closed the tracker merged.
#   related            Issues whose bodies cite a similar but different feature.
#   new                Headlines reconciled a minute before their tracker was filed, when nothing tracked them yet.
#   no-work            SDK release notes that need no provider work.
#
# as_of defaults to "filed" for issue inputs. Regenerate the duplicate sample only with care: labels must not be
# tuned to Radar's output.

target = "tfaws"

# --- duplicate

case "dup-40793" {
  input = "40793"
  kind  = "duplicate"
  note  = "[Bug]: RDS Cluster: Changing cluster_identifier forces resource recreation, while AWS Cons"
  expect {
    number   = 38644
    category = "already_tracked"
  }
}

case "dup-41200" {
  input = "41200"
  kind  = "duplicate"
  note  = "[Bug]: aws_autoscaling_schedule should reset StartTime"
  expect {
    number   = 38983
    category = "already_tracked"
  }
}

case "dup-41552" {
  input = "41552"
  kind  = "duplicate"
  note  = "[Enhancement]: Support Dedicated coordinator nodes in an OpenSearch domain"
  expect {
    number   = 40135
    category = "already_tracked"
  }
}

case "dup-41782" {
  input = "41782"
  kind  = "duplicate"
  note  = "[Bug]: s3_bucket_lifecycle_configuration warning: filter is required?"
  expect {
    number   = 41710
    category = "already_tracked"
  }
}

case "dup-41973" {
  input = "41973"
  kind  = "duplicate"
  note  = "[Bug]: Invalid Attribute Combination with aws_s3_bucket_lifecycle_configuration"
  expect {
    number   = 41710
    category = "already_tracked"
  }
}

case "dup-42127" {
  input = "42127"
  kind  = "duplicate"
  note  = "SSM Document issue regarding delete: InvalidDocument above 5.1.0 provider version"
  expect {
    number   = 42125
    category = "already_tracked"
  }
}

case "dup-42605" {
  input = "42605"
  kind  = "duplicate"
  note  = "aws_sagemaker_image_version id is based only on name, should rely on version too."
  expect {
    number   = 40597
    category = "previously_considered"
  }
}

case "dup-43223" {
  input = "43223"
  kind  = "duplicate"
  note  = "New resource for Security group rule"
  expect {
    number   = 39383
    category = "already_tracked"
  }
}

case "dup-43828" {
  input = "43828"
  kind  = "duplicate"
  note  = "Structured Data Sources For Bedrock Knowledge Bases Through Redshift"
  expect {
    number   = 42850
    category = "already_tracked"
  }
}

case "dup-44751" {
  input = "44751"
  kind  = "duplicate"
  note  = "Add odb_network_arn to aws_route resource and data source"
  expect {
    number   = 44672
    category = "already_tracked"
  }
}

case "dup-45838" {
  input = "45838"
  kind  = "duplicate"
  note  = "Update aws_datazone_environment_blueprint_configuration to support global_parameters"
  expect {
    number   = 44836
    category = "already_tracked"
  }
}

case "dup-46606" {
  input = "46606"
  kind  = "duplicate"
  note  = "aws_dynamodb_table fails with Error: all indexes must match a defined attribute. Unmatched"
  expect {
    number   = 46322
    category = "already_tracked"
  }
}

case "dup-47486" {
  input = "47486"
  kind  = "duplicate"
  note  = "`aws_wafv2_web_acl_rule`: `scope_down_statement` does not support complex blocks (`and_sta"
  expect {
    number   = 47022
    category = "already_tracked"
  }
}

case "dup-47987" {
  input = "47987"
  kind  = "duplicate"
  note  = "Add aws_ecs_task_execution 'action' resource like aws_lambda_invoke"
  expect {
    number   = 44688
    category = "already_tracked"
  }
}

case "dup-48354" {
  input = "48354"
  kind  = "duplicate"
  note  = "aws_cloudfront_distribution_tenant fails with managed_certificate_request"
  expect {
    number   = 45987
    category = "already_tracked"
  }
}

case "dup-48760" {
  input = "48760"
  kind  = "duplicate"
  note  = "enhacement: aws_transfer_web_app should support ip_address_type for vpc endpoint"
  expect {
    number   = 48554
    category = "already_tracked"
  }
}

case "dup-49183" {
  input = "49183"
  kind  = "duplicate"
  note  = "[Enhancement]: Implementation of write-only arguments in opensearch_domain"
  expect {
    number   = 43509
    category = "already_tracked"
  }
}

case "dup-49365" {
  input = "49365"
  kind  = "duplicate"
  note  = "Add plan-time size validation for `aws_iam_policy`"
  expect {
    number   = 45533
    category = "already_tracked"
  }
}

case "dup-49894" {
  input = "49894"
  kind  = "duplicate"
  note  = "[Bug] aws_dynamodb_table rejects a valid warm_throughput value when billing_mode is PROVIS"
  expect {
    number   = 47673
    category = "already_tracked"
  }
}

case "dup-50540" {
  input = "50540"
  kind  = "duplicate"
  note  = "It is not possible to change APNS signing key for token-based auth"
  expect {
    number   = 41838
    category = "already_tracked"
  }
}

# --- announcement and announcement-text

case "ann-42864" {
  input = "https://aws.amazon.com/about-aws/whats-new/2025/06/amazon-api-gateway-routing-rules-rest-apis/"
  as_of = "2025-06-04T13:11:54Z"
  kind  = "announcement"
  note  = "API Gateway routing rules for REST APIs"
  expect {
    number   = 42864
    category = "already_tracked"
  }
}

case "ann-text-42864" {
  text  = "Amazon API Gateway now supports routing rules for REST APIs using custom domain names."
  as_of = "2025-06-04T13:11:54Z"
  kind  = "announcement-text"
  expect {
    number   = 42864
    category = "already_tracked"
  }
}

case "ann-43005" {
  input = "https://aws.amazon.com/about-aws/whats-new/2025/06/aws-waf-automatic-application-layer-ddos-protection/"
  as_of = "2025-06-13T14:34:02Z"
  kind  = "announcement"
  note  = "L7 DDoS protections for AWS WAF"
  expect {
    number   = 43005
    category = "already_tracked"
  }
}

case "ann-text-43005" {
  text  = "AWS WAF now supports automatic application layer distributed denial of service (DDoS) protection"
  as_of = "2025-06-13T14:34:02Z"
  kind  = "announcement-text"
  expect {
    number   = 43005
    category = "already_tracked"
  }
}

case "ann-45162" {
  input = "https://aws.amazon.com/about-aws/whats-new/2025/11/amazon-fsx-windows-file-server-resource-manager/"
  as_of = "2025-11-20T16:56:40Z"
  kind  = "announcement"
  note  = "FSx for Windows File Server File Server Resource Manager"
  expect {
    number   = 45162
    category = "already_tracked"
  }
}

case "ann-text-45162" {
  text  = "Amazon FSx for Windows File Server now supports File Server Resource Manager"
  as_of = "2025-11-20T16:56:40Z"
  kind  = "announcement-text"
  expect {
    number   = 45162
    category = "already_tracked"
  }
}

case "ann-47671" {
  input = "https://aws.amazon.com/about-aws/whats-new/2026/04/aws-kms-tracks-last-usage-kms-keys/"
  as_of = "2026-04-28T17:08:25Z"
  kind  = "announcement"
  note  = "AWS KMS key last usage"
  expect {
    number   = 47671
    category = "already_tracked"
  }
}

case "ann-text-47671" {
  text  = "AWS KMS now tracks last usage of all KMS keys"
  as_of = "2026-04-28T17:08:25Z"
  kind  = "announcement-text"
  expect {
    number   = 47671
    category = "already_tracked"
  }
}

case "ann-49350" {
  input = "https://aws.amazon.com/about-aws/whats-new/2026/08/amazon-dynamodb-vector-search/"
  as_of = "2026-08-07T15:52:22Z"
  kind  = "announcement"
  note  = "Amazon DynamoDB real-time vector search"
  expect {
    number   = 49350
    category = "already_tracked"
  }
}

case "ann-text-49350" {
  text  = "Amazon DynamoDB now supports real-time vector search"
  as_of = "2026-08-07T15:52:22Z"
  kind  = "announcement-text"
  expect {
    number   = 49350
    category = "already_tracked"
  }
}

case "ann-44928" {
  input = "https://aws.amazon.com/about-aws/whats-new/2025/10/amazon-vpc-ipam-prefix-list-automation/"
  as_of = "2025-11-04T21:19:50Z"
  kind  = "announcement"
  note  = "VPC IPAM prefix list updates"
  expect {
    number   = 44928
    category = "already_tracked"
  }
}

case "ann-text-44928" {
  text  = "Today, AWS announced the ability for Amazon VPC IP Address Manager (IPAM) to automate prefix lists updates with prefix list resolver (PLR)."
  as_of = "2025-11-04T21:19:50Z"
  kind  = "announcement-text"
  expect {
    number   = 44928
    category = "already_tracked"
  }
}

# --- implemented

case "impl-45278" {
  input = "https://aws.amazon.com/about-aws/whats-new/2025/11/amazon-cloudwatch-deletion-protection-logs/"
  as_of = "2025-12-03T18:32:01Z"
  kind  = "implemented"
  note  = "CloudWatch log group deletion protection; implemented by #45298"
  expect {
    number   = 45298
    category = "already_implemented"
  }
  expect {
    number = 45278
  }
}

case "impl-45354" {
  input = "https://aws.amazon.com/about-aws/whats-new/2025/12/lambda-durable-multi-step-applications-ai-workflows/"
  as_of = "2025-12-03T17:45:26Z"
  kind  = "implemented"
  note  = "Lambda durable functions; implemented by #45359"
  expect {
    number   = 45359
    category = "already_implemented"
  }
  expect {
    number = 45354
  }
}

case "impl-47389" {
  input = "https://aws.amazon.com/about-aws/whats-new/2026/04/amazon-eks-managed-node-groups-ec2-warm-pools/"
  as_of = "2026-07-22T15:04:21Z"
  kind  = "implemented"
  note  = "EKS managed node groups support EC2 Auto Scaling warm pools; implemented by #48977"
  expect {
    number   = 48977
    category = "already_implemented"
  }
  expect {
    number = 47389
  }
}

case "impl-48487" {
  input = "https://aws.amazon.com/about-aws/whats-new/2026/05/amazon-mq-private-network-connectivity/"
  as_of = "2026-07-02T21:55:52Z"
  kind  = "implemented"
  note  = "Amazon MQ for RabbitMQ private networking; implemented by #48729"
  expect {
    number   = 48729
    category = "already_implemented"
  }
  expect {
    number = 48487
  }
}

case "impl-45014" {
  input = "https://aws.amazon.com/about-aws/whats-new/2025/11/amazon-vpc-lattice-custom-domain-name-resource-configuration/"
  as_of = "2025-11-17T20:21:19Z"
  kind  = "implemented"
  note  = "VPC Lattice custom domain names; implemented by #45085"
  expect {
    number   = 45085
    category = "already_implemented"
  }
  expect {
    number = 45014
  }
}

case "impl-45219" {
  input = "https://aws.amazon.com/about-aws/whats-new/2025/11/announcing-amazon-ecs-express-mode/"
  as_of = "2025-11-26T18:10:18Z"
  kind  = "implemented"
  note  = "ECS Express Mode; implemented by #45235"
  expect {
    number   = 45235
    category = "already_implemented"
  }
  expect {
    number = 45219
  }
}

# --- related: the tracker's "Relates" link is a similar feature, not the same one

case "rel-45362" {
  input        = "45362"
  kind         = "related"
  related_only = [43195]
}

case "rel-47610" {
  input        = "47610"
  kind         = "related"
  related_only = [40964]
}

case "rel-47542" {
  input        = "47542"
  kind         = "related"
  related_only = [47184]
}

case "rel-45361" {
  input        = "45361"
  kind         = "related"
  related_only = [45353]
}

# --- new: headline a minute before the tracker was filed

case "new-45193" {
  text      = "EC2 Auto Scaling introduces instance lifecycle policy"
  as_of     = "2025-11-21T13:50:35Z"
  kind      = "new"
  note      = "tracker #45193 filed a minute later"
  unmatched = true
}

case "new-50573" {
  text      = "AWS announces Amazon Elastic Container Service (Amazon ECS) Gateway, a new resource that simplifies ingress networking for containerized applications by dynamically provisioning and managing Application Load Balancers (ALBs) and their supporting resources directly in your account."
  as_of     = "2026-10-21T12:32:09Z"
  kind      = "new"
  note      = "tracker #50573 filed a minute later"
  unmatched = true
}

case "new-42410" {
  text      = "Today, AWS announced a new AWS Client VPN feature that monitors device networking routes, prevents VPN traffic leaks, and strengthens remote access security."
  as_of     = "2025-04-29T12:14:05Z"
  kind      = "new"
  note      = "tracker #42410 filed a minute later"
  unmatched = true
}

case "new-43082" {
  text      = "Amazon GuardDuty Extended Threat Detection now supports Amazon EKS"
  as_of     = "2025-06-18T19:06:24Z"
  kind      = "new"
  note      = "tracker #43082 filed a minute later"
  unmatched = true
}

case "new-45219" {
  text      = "Today, AWS announces Amazon Elastic Container Service (Amazon ECS) Express Mode, a new feature that empowers developers to rapidly launch containerized applications, including web applications and APIs."
  as_of     = "2025-11-22T21:39:12Z"
  kind      = "new"
  note      = "tracker #45219 filed a minute later"
  unmatched = true
}

# --- no-work: aws-sdk-go-v2 release notes (2026-11-10 and 2026-11-11) with no provider impact

case "nowork-cbor-codebuild" {
  text      = "AWS CodeBuild: This release adds Smithy RPC v2 CBOR as an additional protocol alongside the existing AWS JSON 1.1. The SDK will prioritize its most performant protocol."
  kind      = "no-work"
  unmatched = true
}

case "nowork-schema-serde-iam" {
  text      = "IAM: Enable schema-based (de)serialization for this service."
  kind      = "no-work"
  unmatched = true
}

case "nowork-retryable-conflict" {
  text      = "ARC Region switch: Adds RetryableConflictException to UpdatePlan so AWS SDKs automatically retry updates that conflict with a concurrent change."
  kind      = "no-work"
  unmatched = true
}

case "nowork-model-formatting" {
  text      = "Amazon Verified Permissions: Inconsequential changes to model formatting"
  kind      = "no-work"
  unmatched = true
}

# --- dev: queries used while building milestone 1. Reported separately because the rules were tuned on them.

case "dev-45362-code" {
  input = "45362"
  as_of = "now"
  code  = true
  kind  = "dev"
  expect {
    path     = "internal/service/fsx/s3_access_point_attachment.go"
    category = "already_implemented"
  }
  expect {
    number   = 50872
    category = "previously_considered"
  }
  related_only = [45361]
}

case "dev-45362-url" {
  input = "https://aws.amazon.com/about-aws/whats-new/2025/12/amazon-fsx-netapp-ontap-s3-access/"
  as_of = "now"
  kind  = "dev"
  expect {
    number   = 45362
    category = "already_tracked"
  }
}

case "dev-50921" {
  input = "50921"
  as_of = "now"
  code  = true
  kind  = "dev"
  expect {
    number   = 50924
    category = "already_tracked"
  }
}

case "dev-text-fsx" {
  text      = "FSx S3 access point support"
  kind      = "dev"
  unmatched = true
  expect {
    number   = 45362
    category = "related"
  }
}
