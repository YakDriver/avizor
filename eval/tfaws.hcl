# Radar evaluation corpus for the Terraform AWS Provider.
#
# Every expectation comes from repository history, not from Radar's output:
#   duplicate          GitHub "Duplicate of #N" markings. Every 5th of the 98 pairs filed since 2025-01-01 whose original
#                      predates the duplicate, in issue order.
#   announcement       What's New URLs linked from maintainer tracking issues, reconciled 30 minutes after the tracker was
#                      filed.
#   announcement-text  The same announcements' headlines as free text, as a differently worded request would arrive.
#   implemented        Announcements reconciled an hour after the PR that closed the tracker merged.
#   related            Trackers whose "Relates" link is a similar but different feature.
#   new                Headlines reconciled a minute before their tracker was filed, when nothing tracked them yet.
#   no-work            SDK release notes that need no provider work.
#   item-tracked       Pulled SDK items whose release commit a maintainer tracker (or PR) with the item's service label
#                      cites, reconciled 30 minutes after the later of the release and the last such tracker. Every
#                      such item in releases from 2025-12-01 to 2026-11-11, except ec2/release-2026-09-04, which only
#                      the bump PR cites. Radar matches on the same release commit, so these mostly measure precision
#                      on the other candidates, not retrieval.
#   item-new           The same items reconciled a minute before their first tracker was filed, when it was filed
#                      after the release.
#
#   item-no-work       Pulled SDK items that need no provider work.
#
# Item cases need pulled items: run `radar pull --since 2025-11-30` first.
#
# same and different judge other candidates Radar returns with a confident category (anything above "related"): same
# is the same work as the input, such as a second duplicate or the implementing PR; different is not. Judgments come
# from reading the candidate, not from its category. A confident category on different work fails the case. Run
# `radar eval --format text --unjudged` to list confident candidates that still need a judgment.
#
# Cases with code = true are judged against the checkout as of labeling (provider main at 37436a92a45, 2026-11-12).
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
  different = [33461, 33460, 32914, 26230, 29213, 29753]
}

case "dup-41552" {
  input = "41552"
  kind  = "duplicate"
  note  = "[Enhancement]: Support Dedicated coordinator nodes in an OpenSearch domain"
  expect {
    number   = 40135
    category = "already_tracked"
  }
  same = [40181]
}

case "dup-41782" {
  input = "41782"
  kind  = "duplicate"
  note  = "[Bug]: s3_bucket_lifecycle_configuration warning: filter is required?"
  expect {
    number   = 41710
    category = "already_tracked"
  }
  different = [41462, 31299, 41199, 41126, 41620]
}

case "dup-41973" {
  input = "41973"
  kind  = "duplicate"
  note  = "[Bug]: Invalid Attribute Combination with aws_s3_bucket_lifecycle_configuration"
  expect {
    number   = 41710
    category = "already_tracked"
  }
  same      = [41782]
  different = [41462, 41199, 31299, 41126, 41620]
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
  different = [35931]
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
  different = [41241, 41384]
}

case "dup-44751" {
  input = "44751"
  kind  = "duplicate"
  note  = "Add odb_network_arn to aws_route resource and data source"
  expect {
    number   = 44672
    category = "already_tracked"
  }
  different = [39447]
}

case "dup-45838" {
  input = "45838"
  kind  = "duplicate"
  note  = "Update aws_datazone_environment_blueprint_configuration to support global_parameters"
  expect {
    number   = 44836
    category = "already_tracked"
  }
  different = [43043]
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
  same      = [14619, 33088]
  different = [46315]
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
  input     = "45362"
  kind      = "related"
  different = [43195]
}

case "rel-47610" {
  input     = "47610"
  kind      = "related"
  different = [40964]
}

case "rel-47542" {
  input     = "47542"
  kind      = "related"
  different = [47184]
}

case "rel-45361" {
  input     = "45361"
  kind      = "related"
  different = [45353, 45360]
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
  same       = [50917]
  different  = [45361, 43391, 43195]
  same_paths = ["internal/service/fsx/sweep.go", "website/docs/r/fsx_s3_access_point_attachment.html.markdown"]
}

case "dev-45362-url" {
  input = "https://aws.amazon.com/about-aws/whats-new/2025/12/amazon-fsx-netapp-ontap-s3-access/"
  as_of = "now"
  kind  = "dev"
  expect {
    number   = 45362
    category = "already_tracked"
  }
  same = [50917]
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

# --- item-tracked and item-new

case "item-tracked-acm-2026-06-30" {
  item  = "aws-sdk-go-v2/acm/release-2026-06-30"
  as_of = "2026-07-01T16:43:19Z"
  kind  = "item-tracked"
  note  = "ACM release-2026-06-30: #48727 cite the release commit"
  expect {
    number   = 48727
    category = "already_tracked"
  }
}

case "item-new-acm-2026-06-30" {
  item      = "aws-sdk-go-v2/acm/release-2026-06-30"
  as_of     = "2026-07-01T16:12:19Z"
  kind      = "item-new"
  note      = "tracker #48727 filed a minute later"
  unmatched = true
}

case "item-tracked-agentregistrycontrol-2026-08-06" {
  item  = "aws-sdk-go-v2/agentregistrycontrol/release-2026-08-06"
  as_of = "2026-08-06T19:32:15Z"
  kind  = "item-tracked"
  note  = "Agent Registry Control release-2026-08-06: #48694 cite the release commit"
  expect {
    number   = 48694
    category = "already_tracked"
  }
  different = [48693]
}

case "item-tracked-bedrockagentcorecontrol-2026-08-06" {
  item  = "aws-sdk-go-v2/bedrockagentcorecontrol/release-2026-08-06"
  as_of = "2026-08-07T16:05:46Z"
  kind  = "item-tracked"
  note  = "Bedrock AgentCore Control release-2026-08-06: #48694, #49352 cite the release commit"
  expect {
    number   = 48694
    category = "already_tracked"
  }
  expect {
    number   = 49352
    category = "already_tracked"
  }
  same      = [49344]
  different = [48693]
}

case "item-tracked-cloudformation-2026-06-30" {
  item  = "aws-sdk-go-v2/cloudformation/release-2026-06-30"
  as_of = "2026-07-01T16:48:40Z"
  kind  = "item-tracked"
  note  = "CloudFormation release-2026-06-30: #48728 cite the release commit"
  expect {
    number   = 48728
    category = "already_tracked"
  }
}

case "item-new-cloudformation-2026-06-30" {
  item      = "aws-sdk-go-v2/cloudformation/release-2026-06-30"
  as_of     = "2026-07-01T16:17:40Z"
  kind      = "item-new"
  note      = "tracker #48728 filed a minute later"
  unmatched = true
}

case "item-tracked-cloudwatchlogs-2026-06-22" {
  item  = "aws-sdk-go-v2/cloudwatchlogs/release-2026-06-22"
  as_of = "2026-06-24T14:03:32Z"
  kind  = "item-tracked"
  note  = "CloudWatch Logs release-2026-06-22: #48547 cite the release commit"
  expect {
    number   = 48547
    category = "already_tracked"
  }
}

case "item-new-cloudwatchlogs-2026-06-22" {
  item      = "aws-sdk-go-v2/cloudwatchlogs/release-2026-06-22"
  as_of     = "2026-06-24T13:32:32Z"
  kind      = "item-new"
  note      = "tracker #48547 filed a minute later"
  unmatched = true
}

case "item-tracked-dynamodb-2026-08-04" {
  item  = "aws-sdk-go-v2/dynamodb/release-2026-08-04"
  as_of = "2026-08-07T15:52:22Z"
  kind  = "item-tracked"
  note  = "DynamoDB release-2026-08-04: #49350 cite the release commit"
  expect {
    number   = 49350
    category = "already_tracked"
  }
}

case "item-new-dynamodb-2026-08-04" {
  item      = "aws-sdk-go-v2/dynamodb/release-2026-08-04"
  as_of     = "2026-08-07T15:21:22Z"
  kind      = "item-new"
  note      = "tracker #49350 filed a minute later"
  unmatched = true
}

case "item-tracked-ec2-2025-12-18" {
  item  = "aws-sdk-go-v2/ec2/release-2025-12-18"
  as_of = "2025-12-19T14:03:58Z"
  kind  = "item-tracked"
  note  = "EC2 release-2025-12-18: #45665 cite the release commit"
  expect {
    number   = 45665
    category = "already_tracked"
  }
}

case "item-new-ec2-2025-12-18" {
  item      = "aws-sdk-go-v2/ec2/release-2025-12-18"
  as_of     = "2025-12-19T13:32:58Z"
  kind      = "item-new"
  note      = "tracker #45665 filed a minute later"
  unmatched = true
}

case "item-tracked-ec2-2026-02-12" {
  item  = "aws-sdk-go-v2/ec2/release-2026-02-12"
  as_of = "2026-02-18T03:20:52Z"
  kind  = "item-tracked"
  note  = "EC2 release-2026-02-12: #46533 cite the release commit"
  expect {
    number   = 46533
    category = "already_tracked"
  }
}

case "item-new-ec2-2026-02-12" {
  item      = "aws-sdk-go-v2/ec2/release-2026-02-12"
  as_of     = "2026-02-18T02:49:52Z"
  kind      = "item-new"
  note      = "tracker #46533 filed a minute later"
  unmatched = true
}

case "item-tracked-ec2-2026-04-22" {
  item  = "aws-sdk-go-v2/ec2/release-2026-04-22"
  as_of = "2026-04-23T18:29:47Z"
  kind  = "item-tracked"
  note  = "EC2 release-2026-04-22: #47609 cite the release commit"
  expect {
    number   = 47609
    category = "already_tracked"
  }
}

case "item-new-ec2-2026-04-22" {
  item      = "aws-sdk-go-v2/ec2/release-2026-04-22"
  as_of     = "2026-04-23T17:58:47Z"
  kind      = "item-new"
  note      = "tracker #47609 filed a minute later"
  unmatched = true
}

case "item-tracked-ecs-2026-04-01" {
  item  = "aws-sdk-go-v2/ecs/release-2026-04-01"
  as_of = "2026-04-02T12:36:04Z"
  kind  = "item-tracked"
  note  = "ECS release-2026-04-01: #47200 cite the release commit"
  expect {
    number   = 47200
    category = "already_tracked"
  }
}

case "item-new-ecs-2026-04-01" {
  item      = "aws-sdk-go-v2/ecs/release-2026-04-01"
  as_of     = "2026-04-02T12:05:04Z"
  kind      = "item-new"
  note      = "tracker #47200 filed a minute later"
  unmatched = true
}

case "item-tracked-eks-2025-12-01" {
  item  = "aws-sdk-go-v2/eks/release-2025-12-01"
  as_of = "2025-12-01T07:21:34Z"
  kind  = "item-tracked"
  note  = "EKS release-2025-12-01: #45324 cite the release commit"
  expect {
    number   = 45324
    category = "already_tracked"
  }
  same = [45326]
}

case "item-new-eks-2025-12-01" {
  item      = "aws-sdk-go-v2/eks/release-2025-12-01"
  as_of     = "2025-12-01T06:50:34Z"
  kind      = "item-new"
  note      = "tracker #45324 filed a minute later"
  unmatched = true
}

case "item-tracked-eks-2026-04-07" {
  item  = "aws-sdk-go-v2/eks/release-2026-04-07"
  as_of = "2026-04-09T21:15:30Z"
  kind  = "item-tracked"
  note  = "EKS release-2026-04-07: #47389 cite the release commit"
  expect {
    number   = 47389
    category = "already_tracked"
  }
}

case "item-new-eks-2026-04-07" {
  item      = "aws-sdk-go-v2/eks/release-2026-04-07"
  as_of     = "2026-04-09T20:44:30Z"
  kind      = "item-new"
  note      = "tracker #47389 filed a minute later"
  unmatched = true
}

case "item-tracked-eks-2026-06-18" {
  item  = "aws-sdk-go-v2/eks/release-2026-06-18"
  as_of = "2026-06-20T12:35:01Z"
  kind  = "item-tracked"
  note  = "EKS release-2026-06-18: #48470, #48497 cite the release commit"
  expect {
    number   = 48470
    category = "already_tracked"
  }
  expect {
    number   = 48497
    category = "already_tracked"
  }
}

case "item-new-eks-2026-06-18" {
  item      = "aws-sdk-go-v2/eks/release-2026-06-18"
  as_of     = "2026-06-19T00:00:07Z"
  kind      = "item-new"
  note      = "tracker #48470 filed a minute later"
  unmatched = true
}

case "item-tracked-eks-2026-06-30" {
  item  = "aws-sdk-go-v2/eks/release-2026-06-30"
  as_of = "2026-07-02T14:17:44Z"
  kind  = "item-tracked"
  note  = "EKS release-2026-06-30: #48751 cite the release commit"
  expect {
    number   = 48751
    category = "already_tracked"
  }
}

case "item-new-eks-2026-06-30" {
  item      = "aws-sdk-go-v2/eks/release-2026-06-30"
  as_of     = "2026-07-02T13:46:44Z"
  kind      = "item-new"
  note      = "tracker #48751 filed a minute later"
  unmatched = true
}

case "item-tracked-fsx-2025-12-02" {
  item  = "aws-sdk-go-v2/fsx/release-2025-12-02"
  as_of = "2025-12-02T20:13:26Z"
  kind  = "item-tracked"
  note  = "FSx release-2025-12-02: #45362 cite the release commit"
  expect {
    number   = 45362
    category = "already_tracked"
  }
}

case "item-new-fsx-2025-12-02" {
  item      = "aws-sdk-go-v2/fsx/release-2025-12-02"
  as_of     = "2025-12-02T19:42:26Z"
  kind      = "item-new"
  note      = "tracker #45362 filed a minute later"
  unmatched = true
}

case "item-tracked-grafana-2026-02-17" {
  item  = "aws-sdk-go-v2/grafana/release-2026-02-17"
  as_of = "2026-02-19T14:07:45Z"
  kind  = "item-tracked"
  note  = "grafana release-2026-02-17: #46559 cite the release commit"
  expect {
    number   = 46559
    category = "already_tracked"
  }
}

case "item-new-grafana-2026-02-17" {
  item      = "aws-sdk-go-v2/grafana/release-2026-02-17"
  as_of     = "2026-02-19T13:36:45Z"
  kind      = "item-new"
  note      = "tracker #46559 filed a minute later"
  unmatched = true
}

case "item-tracked-kinesis-2026-08-31" {
  item  = "aws-sdk-go-v2/kinesis/release-2026-08-31"
  as_of = "2026-09-02T13:36:11Z"
  kind  = "item-tracked"
  note  = "Kinesis release-2026-08-31: #49792 cite the release commit"
  expect {
    number   = 49792
    category = "already_tracked"
  }
}

case "item-new-kinesis-2026-08-31" {
  item      = "aws-sdk-go-v2/kinesis/release-2026-08-31"
  as_of     = "2026-09-02T13:05:11Z"
  kind      = "item-new"
  note      = "tracker #49792 filed a minute later"
  unmatched = true
}

case "item-tracked-kms-2026-04-27" {
  item  = "aws-sdk-go-v2/kms/release-2026-04-27"
  as_of = "2026-04-28T17:08:25Z"
  kind  = "item-tracked"
  note  = "KMS release-2026-04-27: #47671 cite the release commit"
  expect {
    number   = 47671
    category = "already_tracked"
  }
}

case "item-new-kms-2026-04-27" {
  item      = "aws-sdk-go-v2/kms/release-2026-04-27"
  as_of     = "2026-04-28T16:37:25Z"
  kind      = "item-new"
  note      = "tracker #47671 filed a minute later"
  unmatched = true
}

case "item-tracked-lambda-2025-12-01" {
  item  = "aws-sdk-go-v2/lambda/release-2025-12-01"
  as_of = "2025-12-01T13:34:01Z"
  kind  = "item-tracked"
  note  = "Lambda release-2025-12-01: #45332 cite the release commit"
  expect {
    number   = 45332
    category = "already_tracked"
  }
}

case "item-new-lambda-2025-12-01" {
  item      = "aws-sdk-go-v2/lambda/release-2025-12-01"
  as_of     = "2025-12-01T13:03:01Z"
  kind      = "item-new"
  note      = "tracker #45332 filed a minute later"
  unmatched = true
}

case "item-tracked-lambda-2025-12-02" {
  item  = "aws-sdk-go-v2/lambda/release-2025-12-02"
  as_of = "2025-12-02T17:04:38Z"
  kind  = "item-tracked"
  note  = "Lambda release-2025-12-02: #45354 cite the release commit"
  expect {
    number   = 45354
    category = "already_tracked"
  }
}

case "item-tracked-lambdacore-2026-06-22" {
  item  = "aws-sdk-go-v2/lambdacore/release-2026-06-22"
  as_of = "2026-06-23T13:58:51Z"
  kind  = "item-tracked"
  note  = "Lambda Core release-2026-06-22: #48526 cite the release commit"
  expect {
    number   = 48526
    category = "already_tracked"
  }
}

case "item-new-lambdacore-2026-06-22" {
  item      = "aws-sdk-go-v2/lambdacore/release-2026-06-22"
  as_of     = "2026-06-23T13:27:51Z"
  kind      = "item-new"
  note      = "tracker #48526 filed a minute later"
  unmatched = true
}

case "item-tracked-lambdamicrovms-2026-06-22" {
  item  = "aws-sdk-go-v2/lambdamicrovms/release-2026-06-22"
  as_of = "2026-06-23T13:58:51Z"
  kind  = "item-tracked"
  note  = "Lambda Microvms release-2026-06-22: #48526 cite the release commit"
  expect {
    number   = 48526
    category = "already_tracked"
  }
}

case "item-new-lambdamicrovms-2026-06-22" {
  item      = "aws-sdk-go-v2/lambdamicrovms/release-2026-06-22"
  as_of     = "2026-06-23T13:27:51Z"
  kind      = "item-new"
  note      = "tracker #48526 filed a minute later"
  unmatched = true
}

case "item-tracked-lambdaweb-2026-10-12" {
  item  = "aws-sdk-go-v2/lambdaweb/release-2026-10-12"
  as_of = "2026-10-13T01:48:51Z"
  kind  = "item-tracked"
  note  = "Lambda Web release-2026-10-12: #50429 cite the release commit"
  expect {
    number   = 50429
    category = "already_tracked"
  }
  same = [50341]
}

case "item-new-lambdaweb-2026-10-12" {
  item      = "aws-sdk-go-v2/lambdaweb/release-2026-10-12"
  as_of     = "2026-10-13T01:17:51Z"
  kind      = "item-new"
  note      = "tracker #50429 filed a minute later"
  unmatched = true
}

case "item-tracked-mq-2026-06-17" {
  item  = "aws-sdk-go-v2/mq/release-2026-06-17"
  as_of = "2026-06-19T14:09:53Z"
  kind  = "item-tracked"
  note  = "mq release-2026-06-17: #48487 cite the release commit"
  expect {
    number   = 48487
    category = "already_tracked"
  }
}

case "item-new-mq-2026-06-17" {
  item      = "aws-sdk-go-v2/mq/release-2026-06-17"
  as_of     = "2026-06-19T13:38:53Z"
  kind      = "item-new"
  note      = "tracker #48487 filed a minute later"
  unmatched = true
}

case "item-tracked-mq-2026-07-14" {
  item  = "aws-sdk-go-v2/mq/release-2026-07-14"
  as_of = "2026-07-17T19:06:10Z"
  kind  = "item-tracked"
  note  = "mq release-2026-07-14: #49008 cite the release commit"
  expect {
    number   = 49008
    category = "already_tracked"
  }
  same = [49010]
}

case "item-new-mq-2026-07-14" {
  item      = "aws-sdk-go-v2/mq/release-2026-07-14"
  as_of     = "2026-07-17T18:35:10Z"
  kind      = "item-new"
  note      = "tracker #49008 filed a minute later"
  unmatched = true
}

case "item-tracked-networkmanager-2026-10-22" {
  item  = "aws-sdk-go-v2/networkmanager/release-2026-10-22"
  as_of = "2026-10-23T18:05:51Z"
  kind  = "item-tracked"
  note  = "NetworkManager release-2026-10-22: #50613 cite the release commit"
  expect {
    number   = 50613
    category = "already_tracked"
  }
}

case "item-new-networkmanager-2026-10-22" {
  item      = "aws-sdk-go-v2/networkmanager/release-2026-10-22"
  as_of     = "2026-10-23T17:34:51Z"
  kind      = "item-new"
  note      = "tracker #50613 filed a minute later"
  unmatched = true
}

case "item-tracked-odb-2026-08-12" {
  item  = "aws-sdk-go-v2/odb/release-2026-08-12"
  as_of = "2026-08-13T14:02:35Z"
  kind  = "item-tracked"
  note  = "odb release-2026-08-12: #49464 cite the release commit"
  expect {
    number   = 49464
    category = "already_tracked"
  }
}

case "item-new-odb-2026-08-12" {
  item      = "aws-sdk-go-v2/odb/release-2026-08-12"
  as_of     = "2026-08-13T13:31:35Z"
  kind      = "item-new"
  note      = "tracker #49464 filed a minute later"
  unmatched = true
}

case "item-tracked-opensearch-2026-07-01" {
  item  = "aws-sdk-go-v2/opensearch/release-2026-07-01"
  as_of = "2026-07-02T14:12:32Z"
  kind  = "item-tracked"
  note  = "OpenSearch release-2026-07-01: #48750 cite the release commit"
  expect {
    number   = 48750
    category = "already_tracked"
  }
}

case "item-new-opensearch-2026-07-01" {
  item      = "aws-sdk-go-v2/opensearch/release-2026-07-01"
  as_of     = "2026-07-02T13:41:32Z"
  kind      = "item-new"
  note      = "tracker #48750 filed a minute later"
  unmatched = true
}

case "item-tracked-rds-2026-03-24" {
  item  = "aws-sdk-go-v2/rds/release-2026-03-24"
  as_of = "2026-03-26T12:40:08Z"
  kind  = "item-tracked"
  note  = "RDS release-2026-03-24: #47117 cite the release commit"
  expect {
    number   = 47117
    category = "already_tracked"
  }
}

case "item-new-rds-2026-03-24" {
  item      = "aws-sdk-go-v2/rds/release-2026-03-24"
  as_of     = "2026-03-26T12:09:08Z"
  kind      = "item-new"
  note      = "tracker #47117 filed a minute later"
  unmatched = true
}

case "item-tracked-resiliencehubv2-2026-05-28" {
  item  = "aws-sdk-go-v2/resiliencehubv2/release-2026-05-28"
  as_of = "2026-05-29T14:37:02Z"
  kind  = "item-tracked"
  note  = "resiliencehubv2 release-2026-05-28: #48122 cite the release commit"
  expect {
    number   = 48122
    category = "already_tracked"
  }
}

case "item-new-resiliencehubv2-2026-05-28" {
  item      = "aws-sdk-go-v2/resiliencehubv2/release-2026-05-28"
  as_of     = "2026-05-29T14:06:02Z"
  kind      = "item-new"
  note      = "tracker #48122 filed a minute later"
  unmatched = true
}

case "item-tracked-s3-2026-03-12" {
  item  = "aws-sdk-go-v2/s3/release-2026-03-12"
  as_of = "2026-03-12T20:26:12Z"
  kind  = "item-tracked"
  note  = "S3 release-2026-03-12: #46902 cite the release commit"
  expect {
    number   = 46902
    category = "already_tracked"
  }
}

case "item-new-s3-2026-03-12" {
  item      = "aws-sdk-go-v2/s3/release-2026-03-12"
  as_of     = "2026-03-12T19:55:12Z"
  kind      = "item-new"
  note      = "tracker #46902 filed a minute later"
  unmatched = true
}

case "item-tracked-s3-2026-04-22" {
  item  = "aws-sdk-go-v2/s3/release-2026-04-22"
  as_of = "2026-04-23T18:36:08Z"
  kind  = "item-tracked"
  note  = "S3 release-2026-04-22: #47610 cite the release commit"
  expect {
    number   = 47610
    category = "already_tracked"
  }
}

case "item-new-s3-2026-04-22" {
  item      = "aws-sdk-go-v2/s3/release-2026-04-22"
  as_of     = "2026-04-23T18:05:08Z"
  kind      = "item-new"
  note      = "tracker #47610 filed a minute later"
  unmatched = true
}

case "item-tracked-s3-2026-06-16" {
  item  = "aws-sdk-go-v2/s3/release-2026-06-16"
  as_of = "2026-06-17T17:06:21Z"
  kind  = "item-tracked"
  note  = "S3 release-2026-06-16: #48444 cite the release commit"
  expect {
    number   = 48444
    category = "already_tracked"
  }
}

case "item-new-s3-2026-06-16" {
  item      = "aws-sdk-go-v2/s3/release-2026-06-16"
  as_of     = "2026-06-17T16:35:21Z"
  kind      = "item-new"
  note      = "tracker #48444 filed a minute later"
  unmatched = true
}

case "item-tracked-s3-2026-08-06" {
  item  = "aws-sdk-go-v2/s3/release-2026-08-06"
  as_of = "2026-08-07T15:58:51Z"
  kind  = "item-tracked"
  note  = "S3 release-2026-08-06: #49351 cite the release commit"
  expect {
    number   = 49351
    category = "already_tracked"
  }
}

case "item-new-s3-2026-08-06" {
  item      = "aws-sdk-go-v2/s3/release-2026-08-06"
  as_of     = "2026-08-07T15:27:51Z"
  kind      = "item-new"
  note      = "tracker #49351 filed a minute later"
  unmatched = true
}

case "item-tracked-s3-2026-09-08" {
  item  = "aws-sdk-go-v2/s3/release-2026-09-08"
  as_of = "2026-09-09T12:27:55Z"
  kind  = "item-tracked"
  note  = "S3 release-2026-09-08: #49909 cite the release commit"
  expect {
    number   = 49909
    category = "already_tracked"
  }
}

case "item-new-s3-2026-09-08" {
  item      = "aws-sdk-go-v2/s3/release-2026-09-08"
  as_of     = "2026-09-09T11:56:55Z"
  kind      = "item-new"
  note      = "tracker #49909 filed a minute later"
  unmatched = true
}

case "item-tracked-s3control-2025-12-02" {
  item  = "aws-sdk-go-v2/s3control/release-2025-12-02"
  as_of = "2025-12-02T20:13:26Z"
  kind  = "item-tracked"
  note  = "S3 Control release-2025-12-02: #45362 cite the release commit"
  expect {
    number   = 45362
    category = "already_tracked"
  }
}

case "item-new-s3control-2025-12-02" {
  item      = "aws-sdk-go-v2/s3control/release-2025-12-02"
  as_of     = "2025-12-02T19:42:26Z"
  kind      = "item-new"
  note      = "tracker #45362 filed a minute later"
  unmatched = true
}

case "item-tracked-s3control-2026-09-08" {
  item  = "aws-sdk-go-v2/s3control/release-2026-09-08"
  as_of = "2026-09-09T12:27:55Z"
  kind  = "item-tracked"
  note  = "S3 Control release-2026-09-08: #49909 cite the release commit"
  expect {
    number   = 49909
    category = "already_tracked"
  }
}

case "item-new-s3control-2026-09-08" {
  item      = "aws-sdk-go-v2/s3control/release-2026-09-08"
  as_of     = "2026-09-09T11:56:55Z"
  kind      = "item-new"
  note      = "tracker #49909 filed a minute later"
  unmatched = true
}

case "item-tracked-s3tables-2025-12-02" {
  item  = "aws-sdk-go-v2/s3tables/release-2025-12-02"
  as_of = "2025-12-02T19:49:01Z"
  kind  = "item-tracked"
  note  = "S3Tables release-2025-12-02: #45353, #45361 cite the release commit"
  expect {
    number   = 45353
    category = "already_tracked"
  }
  expect {
    number   = 45361
    category = "already_tracked"
  }
  same = [45360]
}

# --- item-no-work

case "item-nowork-ec2-2026-09-04" {
  item      = "aws-sdk-go-v2/ec2/release-2026-09-04"
  as_of     = "2026-09-10"
  kind      = "item-no-work"
  note      = "ValidateSecurityGroupQuotasForInterface is for AWS services only; the bump PR #49854 cites the release commit and has the ec2 label"
  unmatched = true
  different = [49854]
}
