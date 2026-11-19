# Avizor evaluation corpus for the Terraform AWS Provider.
#
# Every expectation comes from repository history, not from Avizor's output:
#   duplicate          GitHub "Duplicate of #N" markings. Every 5th of the 98 pairs filed since 2025-01-01 whose original
#                      predates the duplicate, in issue order.
#   announcement       What's New URLs linked from maintainer tracking issues, reconciled 30 minutes after the tracker was
#                      filed.
#   announcement-text  The same announcements' headlines as free text, as a differently worded request would arrive.
#   implemented        Announcements reconciled an hour after the PR that closed the tracker merged.
#   related            Trackers whose "Relates" link is a similar but different feature.
#   new                Headlines reconciled a minute before their tracker was filed, when nothing tracked them yet.
#   no-work            SDK release notes that need no provider work.
#   blog               Blog posts linked from provider issues, each mapped by hand to the item the issue tracks.
#   item-tracked       Pulled SDK items whose release commit a maintainer tracker (or PR) with the item's service label
#                      cites, reconciled 30 minutes after the later of the release and the last such tracker. Every
#                      such item in releases from 2025-12-01 to 2026-11-11, except ec2/release-2026-09-04, which only
#                      the bump PR cites. Avizor matches on the same release commit, so these mostly measure precision
#                      on the other candidates, not retrieval.
#   item-new           The same items reconciled a minute before their first tracker was filed, when it was filed
#                      after the release.
#
#   item-no-work       Pulled SDK items that need no provider work.
#   item-gap           Pulled SDK items that need provider work nothing covers yet, from the gap sample (below).
#   item-covered       Pulled SDK items from the gap sample that existing work covers, though no tracker cites the
#                      release commit.
#   item-covered-sample  The covered sample (below): queue state and judged confident candidates.
#   item-partial         Covered items the first partial-state run flagged, outside the covered sample, hand-checked:
#                        partial when some work has no code or issue, covered when the launch's tracker covers it.
#                        AgentCore release-2026-08-06 and Resilience Hub release-2026-05-28 are left out: each has a
#                        release tracker that may cover the flagged work.
#   post-covered       Blog posts from the 2026-11-19 coverage pass with existing issues or PRs: labeled in the pass,
#                      decided as duplicates, or linked from an issue. Covered by an expected candidate as of that day.
#   post-gap           Posts from the same pass with work nothing covered, reconciled before the gap issues were filed
#                      (2026-11-19T19:15Z). They should be open and significant.
#   post-no-work       Posts from the same pass decided as having no Terraform work. They measure significance
#                      precision; a significant one doesn't fail.
#
# The gap sample: 32 items drafted in a seeded random order (seed 20261112) from items in implemented services with
# the SDK version available and some inferred work, until 30 had no confident existing work. Each was hand-checked
# against provider code at 258e7008847 (2026-11-12) and a GitHub search of issues and PRs.
#
# The covered sample: 30 of the 61 covered items that are not other item cases, in a seeded random order (seed 13),
# hand-checked on 2026-11-13 against provider code at 873712b39f8 and the index. Two (Network Firewall
# release-2026-11-10 and DevOps Agent release-2026-03-30) were first labeled covered and relabeled partial after the
# partial state flagged them; see the design doc.

# Item cases need pulled items: run `avizor pull --since 2025-11-30` first.
#
# same and different judge other candidates Avizor returns with a confident category (anything above "related"): same
# is the same work as the input, such as a second duplicate or the implementing PR; different is not. Judgments come
# from reading the candidate, not from its category. A confident category on different work fails the case. Run
# `avizor eval --format text --unjudged` to list confident candidates that still need a judgment.
#
# Cases with code = true are judged against the checkout as of labeling (provider main at 37436a92a45, 2026-11-12).
#
# as_of defaults to "filed" for issue inputs. Regenerate the duplicate sample only with care: labels must not be
# tuned to Avizor's output.

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
  different = [33461, 33460, 32914, 26230, 29213, 29753, 41132, 41169, 18396, 18395, 18098]
  same = [723]
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
  different = [35628]
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
  different = [39447, 14864, 13888]
}

case "dup-45838" {
  input = "45838"
  kind  = "duplicate"
  note  = "Update aws_datazone_environment_blueprint_configuration to support global_parameters"
  expect {
    number   = 44836
    category = "already_tracked"
  }
  different = [43043, 38345, 38231]
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
  same = [46510]
}

case "item-new-ec2-2026-02-12" {
  item      = "aws-sdk-go-v2/ec2/release-2026-02-12"
  as_of     = "2026-02-17T09:16:00Z"
  kind      = "item-new"
  note      = "request #46510 filed a minute later (2026-02-17T09:17:07Z); tracker #46533 on 2026-02-18"
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

# --- gap sample: item-gap, item-covered, and item-no-work

case "item-nowork-mediaconvert-2026-07-01" {
  item  = "aws-sdk-go-v2/mediaconvert/release-2026-07-01"
  as_of = "2026-11-12T21:00:00Z"
  code  = true
  kind  = "item-no-work"
  note  = "job template and preset settings; the provider has no job template or preset resource (#47145 tracks one)"
  unmatched = true
}

case "item-nowork-gamelift-2026-06-18" {
  item  = "aws-sdk-go-v2/gamelift/release-2026-06-18"
  as_of = "2026-11-12T21:00:00Z"
  code  = true
  kind  = "item-no-work"
  note  = "container group definitions, which the provider does not implement"
  unmatched = true
}

case "item-nowork-connect-2026-01-27" {
  item  = "aws-sdk-go-v2/connect/release-2026-01-27"
  as_of = "2026-11-12T21:00:00Z"
  code  = true
  kind  = "item-no-work"
  note  = "StartTaskContact starts a contact, not a resource"
  unmatched = true
}

case "item-nowork-cleanrooms-2026-07-27" {
  item  = "aws-sdk-go-v2/cleanrooms/release-2026-07-27"
  as_of = "2026-11-12T21:00:00Z"
  code  = true
  kind  = "item-no-work"
  note  = "worker type for protected queries, which are not resources"
  unmatched = true
}

case "item-nowork-sagemaker-2026-03-11" {
  item  = "aws-sdk-go-v2/sagemaker/release-2026-03-11"
  as_of = "2026-11-12T21:00:00Z"
  code  = true
  kind  = "item-no-work"
  note  = "training plan extensions; the provider has no training plan resource"
  unmatched = true
}

case "item-nowork-bcmdataexports-2026-07-27" {
  item  = "aws-sdk-go-v2/bcmdataexports/release-2026-07-27"
  as_of = "2026-11-12T21:00:00Z"
  code  = true
  kind  = "item-no-work"
  note  = "new enum values; aws_bcmdataexports_export validates against the SDK enum, so the bump covers it"
  unmatched = true
}

case "item-nowork-glue-2026-10-12" {
  item  = "aws-sdk-go-v2/glue/release-2026-10-12"
  as_of = "2026-11-12T21:00:00Z"
  code  = true
  kind  = "item-no-work"
  note  = "data quality result V2 reads and publishes; no resource to manage"
  unmatched = true
}

case "item-nowork-cloudformation-2026-07-31-2" {
  item  = "aws-sdk-go-v2/cloudformation/release-2026-07-31.2"
  as_of = "2026-11-12T21:00:00Z"
  code  = true
  kind  = "item-no-work"
  note  = "an output-only DescribeChangeSet enum value"
  unmatched = true
}

case "item-nowork-sagemaker-2026-04-10" {
  item  = "aws-sdk-go-v2/sagemaker/release-2026-04-10"
  as_of = "2026-11-12T21:00:00Z"
  code  = true
  kind  = "item-no-work"
  note  = "StartClusterHealthCheck and HyperPod node additions, not resource configuration"
  unmatched = true
}

case "item-nowork-connect-2025-12-29" {
  item  = "aws-sdk-go-v2/connect/release-2025-12-29"
  as_of = "2026-11-12T21:00:00Z"
  code  = true
  kind  = "item-no-work"
  note  = "DescribeContact output; contacts are not resources"
  unmatched = true
}

case "item-nowork-codecommit-2026-08-13" {
  item  = "aws-sdk-go-v2/codecommit/release-2026-08-13"
  as_of = "2026-11-12T21:00:00Z"
  code  = true
  kind  = "item-no-work"
  note  = "GetBlobDifferences compares commits; not a resource or a useful data source"
  unmatched = true
}

case "item-nowork-glue-2026-05-14" {
  item  = "aws-sdk-go-v2/glue/release-2026-05-14"
  as_of = "2026-11-12T21:00:00Z"
  code  = true
  kind  = "item-no-work"
  note  = "a GetCatalogs filter"
  unmatched = true
}

case "item-nowork-datazone-2026-09-30" {
  item  = "aws-sdk-go-v2/datazone/release-2026-09-30"
  as_of = "2026-11-12T21:00:00Z"
  code  = true
  kind  = "item-no-work"
  note  = "notebook runs, which are not resources"
  unmatched = true
}

case "item-nowork-sagemaker-2026-08-07" {
  item  = "aws-sdk-go-v2/sagemaker/release-2026-08-07"
  as_of = "2026-11-12T21:00:00Z"
  code  = true
  kind  = "item-no-work"
  note  = "new notebook instance statuses; the waiter targets InService and Stopped"
  unmatched = true
}

case "item-nowork-bedrock-2026-05-14" {
  item  = "aws-sdk-go-v2/bedrock/release-2026-05-14"
  as_of = "2026-11-12T21:00:00Z"
  code  = true
  kind  = "item-no-work"
  note  = "advanced prompt optimization jobs, which run once"
  unmatched = true
}

case "item-nowork-bedrock-2026-05-28" {
  item  = "aws-sdk-go-v2/bedrock/release-2026-05-28"
  as_of = "2026-11-12T21:00:00Z"
  code  = true
  kind  = "item-no-work"
  note  = "CreateCustomModel, which the provider does not call"
  unmatched = true
}

case "item-nowork-sagemaker-2026-05-19" {
  item  = "aws-sdk-go-v2/sagemaker/release-2026-05-19"
  as_of = "2026-11-12T21:00:00Z"
  code  = true
  kind  = "item-no-work"
  note  = "new instance types; aws_sagemaker_notebook_instance does not validate instance_type"
  unmatched = true
}

case "item-nowork-ec2-2026-06-02" {
  item  = "aws-sdk-go-v2/ec2/release-2026-06-02"
  as_of = "2026-11-12T21:00:00Z"
  code  = true
  kind  = "item-no-work"
  note  = "capacity reservation cancellation quotes, which are transient"
  unmatched = true
}

case "item-gap-docdb-2026-09-21" {
  item  = "aws-sdk-go-v2/docdb/release-2026-09-21"
  as_of = "2026-11-12T21:00:00Z"
  code  = true
  kind  = "item-gap"
  note  = "aws_docdb_cluster has no copy_tags_to_snapshot"
  unmatched = true
}

case "item-gap-batch-2026-09-30" {
  item  = "aws-sdk-go-v2/batch/release-2026-09-30"
  as_of = "2026-11-12T21:00:00Z"
  code  = true
  kind  = "item-gap"
  note  = "aws_batch_compute_environment eks_configuration has no access entry"
  unmatched = true
}

case "item-gap-medialive-2026-06-10" {
  item  = "aws-sdk-go-v2/medialive/release-2026-06-10"
  as_of = "2026-11-12T21:00:00Z"
  code  = true
  kind  = "item-gap"
  note  = "aws_medialive_channel has no premix or peak limiter settings"
  unmatched = true
}

case "item-gap-configservice-2026-07-02" {
  item  = "aws-sdk-go-v2/configservice/release-2026-07-02"
  as_of = "2026-11-12T21:00:00Z"
  code  = true
  kind  = "item-gap"
  note  = "the organization rule and conformance pack resources have no tags"
  unmatched = true
}

case "item-gap-quicksight-2026-05-29" {
  item  = "aws-sdk-go-v2/quicksight/release-2026-05-29"
  as_of = "2026-11-12T21:00:00Z"
  code  = true
  kind  = "item-gap"
  note  = "no OAuth client application resource"
  unmatched = true
}

case "item-gap-kafkaconnect-2026-10-22" {
  item  = "aws-sdk-go-v2/kafkaconnect/release-2026-10-22"
  as_of = "2026-11-12T21:00:00Z"
  code  = true
  kind  = "item-gap"
  note  = "aws_mskconnect_connector cannot pause or resume, and has no skip_health_validation"
  unmatched = true
  different = [50823]
}

case "item-gap-rds-2026-08-27" {
  item  = "aws-sdk-go-v2/rds/release-2026-08-27"
  as_of = "2026-11-12T21:00:00Z"
  code  = true
  kind  = "item-gap"
  note  = "aws_db_snapshot has no full snapshot size attribute"
  unmatched = true
}

case "item-gap-appstream-2026-06-19" {
  item  = "aws-sdk-go-v2/appstream/release-2026-06-19"
  as_of = "2026-11-12T21:00:00Z"
  code  = true
  kind  = "item-gap"
  note  = "aws_appstream_stack has no agent access configuration at all"
  unmatched = true
}

case "item-covered-arcregionswitch-2026-02-27" {
  item  = "aws-sdk-go-v2/arcregionswitch/release-2026-02-27"
  as_of = "2026-11-12T21:00:00Z"
  code  = true
  kind  = "item-covered"
  note  = "RDS read replica execution blocks were added to plan.go on 2026-03-16 (PR #49675)"
  expect {
    path     = "internal/service/arcregionswitch/plan.go"
    category = "already_implemented"
  }
  same = [46965]
}

case "item-covered-bedrockagent-2026-08-28-2" {
  item  = "aws-sdk-go-v2/bedrockagent/release-2026-08-28.2"
  as_of = "2026-11-12T21:00:00Z"
  code  = true
  kind  = "item-covered"
  note  = "sync_schedule was added to aws_bedrockagent_data_source in 791712d0633 (2026-10-09)"
  expect {
    path     = "internal/service/bedrockagent/data_source.go"
    category = "already_implemented"
  }
  same = [49958, 49888]
}

case "item-covered-cloudwatch-2026-05-01" {
  item  = "aws-sdk-go-v2/cloudwatch/release-2026-05-01"
  as_of = "2026-11-12T21:00:00Z"
  code  = true
  kind  = "item-covered"
  note  = "#48548 r/aws_cloudwatch_dashboard: Tagging support"
  expect {
    number   = 48548
    category = "already_tracked"
  }
}

case "item-covered-networkfirewall-2026-10-21" {
  item  = "aws-sdk-go-v2/networkfirewall/release-2026-10-21"
  as_of = "2026-11-16T12:00:00Z"
  code  = true
  kind  = "item-covered"
  note  = "#50592 merged route53_resolved_ip_validation; names the member, not an operation (#23)"
  expect {
    number   = 50592
    category = "already_implemented"
  }
  same = [50582, 50589]
  different = [50901]
}

case "item-covered-cloudwatch-2026-10-19" {
  item  = "aws-sdk-go-v2/cloudwatch/release-2026-10-19"
  as_of = "2026-11-16T12:00:00Z"
  code  = true
  kind  = "item-covered"
  note  = "#50590 and PR #50742 name ActionConfiguration / action_configuration (#23)"
  expect {
    number   = 50590
    category = "already_tracked"
  }
  expect {
    number   = 50742
    category = "already_tracked"
  }
}

case "item-covered-dynamodb-2026-10-20" {
  item  = "aws-sdk-go-v2/dynamodb/release-2026-10-20"
  as_of = "2026-11-16T12:00:00Z"
  code  = true
  kind  = "item-covered"
  note  = "#50616 links the matched announcement and names disassociation_protection_enabled (#23)"
  expect {
    number   = 50616
    category = "already_tracked"
  }
  same = [50619, 50656]
  different = [50777]
}

case "item-covered-ec2-2026-10-22" {
  item  = "aws-sdk-go-v2/ec2/release-2026-10-22"
  as_of = "2026-11-17T12:00:00Z"
  code  = true
  kind  = "item-covered"
  note  = "tracker #50613 and its PRs carry service/transitgateway, an ec2 sub-service (#22); the Cloud WAN PRs are the networkmanager item's work"
  expect {
    number   = 50613
    category = "already_tracked"
  }
  same = [50669, 50672, 50683]
  different = [50689, 50690, 50700]
}

case "item-covered-cognitoidentityprovider-2026-06-01" {
  item  = "aws-sdk-go-v2/cognitoidentityprovider/release-2026-06-01"
  as_of = "2026-11-12T21:00:00Z"
  code  = true
  kind  = "item-covered"
  note  = "open PR #48670 adds aws_cognito_user_pool_replica"
  expect {
    number   = 48670
    category = "already_tracked"
  }
  same = [48227]
}

# --- covered sample: item-covered-sample and item-partial

case "item-covered-sample-accountaccess-2026-08-11" {
  item  = "aws-sdk-go-v2/accountaccess/release-2026-08-11"
  as_of = "2026-11-13T19:00:00Z"
  code  = true
  kind  = "item-covered-sample"
  note  = "existing code, issues, or PRs cover the work"
  state = "covered"
  same  = [49553]
  same_paths = [
    "internal/service/accountaccess/application.go",
    "internal/service/accountaccess/application_data_source.go",
    "internal/service/accountaccess/entitlement.go",
    "internal/service/accountaccess/sweep.go",
  ]
}

case "item-covered-sample-athena-2026-11-06" {
  item  = "aws-sdk-go-v2/athena/release-2026-11-06"
  as_of = "2026-11-13T19:00:00Z"
  code  = true
  kind  = "item-covered-sample"
  note  = "covered by #50887 and #50893; SystemTableStatuses is documented as read-only and ignored on write, so it isn't an argument (#25; relabeled from partial after seeing output)"
  state = "covered"
  same  = [50887, 50893]
}

case "item-covered-sample-bedrockagentcorecontrol-2026-02-05" {
  item  = "aws-sdk-go-v2/bedrockagentcorecontrol/release-2026-02-05"
  as_of = "2026-11-13T19:00:00Z"
  code  = true
  kind  = "item-covered-sample"
  note  = "existing code, issues, or PRs cover the work"
  state = "covered"
  same_paths = [
    "internal/service/bedrockagentcore/browser_profile.go",
  ]
}

case "item-covered-sample-bedrockagentcorecontrol-2026-04-22" {
  item  = "aws-sdk-go-v2/bedrockagentcorecontrol/release-2026-04-22"
  as_of = "2026-11-13T19:00:00Z"
  code  = true
  kind  = "item-covered-sample"
  note  = "existing code, issues, or PRs cover the work"
  state = "covered"
  different = [48496, 48654, 49749, 50088, 50654, 50663]
  same_paths = [
    "internal/service/bedrockagentcore/harness.go",
  ]
}

case "item-covered-sample-bedrockagentcorecontrol-2026-09-03" {
  item  = "aws-sdk-go-v2/bedrockagentcorecontrol/release-2026-09-03"
  as_of = "2026-11-13T19:00:00Z"
  code  = true
  kind  = "item-covered-sample"
  note  = "online evaluation config log group prefixes, metrics namespace, and result destination have no code or issue"
  state = "partial"
  same  = [49882, 49947]
}

case "item-covered-sample-bedrockdataautomation-2026-04-02" {
  item  = "aws-sdk-go-v2/bedrockdataautomation/release-2026-04-02"
  as_of = "2026-11-13T19:00:00Z"
  code  = true
  kind  = "item-covered-sample"
  note  = "existing code, issues, or PRs cover the work"
  state = "covered"
  same  = [50727]
  same_paths = [
    "internal/service/bedrockdataautomation/library.go",
  ]
}

case "item-covered-sample-cloudwatch-2026-02-24" {
  item  = "aws-sdk-go-v2/cloudwatch/release-2026-02-24"
  as_of = "2026-11-13T19:00:00Z"
  code  = true
  kind  = "item-covered-sample"
  note  = "existing code, issues, or PRs cover the work"
  state = "covered"
  same  = [46750]
  same_paths = [
    "internal/service/cloudwatch/alarm_mute_rule.go",
  ]
}

case "item-covered-sample-cloudwatch-2026-04-02" {
  item  = "aws-sdk-go-v2/cloudwatch/release-2026-04-02"
  as_of = "2026-11-13T19:00:00Z"
  code  = true
  kind  = "item-covered-sample"
  note  = "existing code, issues, or PRs cover the work"
  state = "covered"
  same  = [50390, 47446, 47449]
  same_paths = [
    "internal/service/cloudwatch/otel_enrichment.go",
  ]
}

case "item-covered-sample-cloudwatchlogs-2025-12-02" {
  item  = "aws-sdk-go-v2/cloudwatchlogs/release-2025-12-02"
  as_of = "2026-11-13T19:00:00Z"
  code  = true
  kind  = "item-covered-sample"
  note  = "existing code, issues, or PRs cover the work"
  state = "covered"
  same  = [48137, 48190]
  different = [48286]
  same_paths = [
    "internal/service/logs/s3_table_integration_source.go",
  ]
}

case "item-covered-sample-cloudwatchomni-2026-09-22" {
  item  = "aws-sdk-go-v2/cloudwatchomni/release-2026-09-22"
  as_of = "2026-11-13T19:00:00Z"
  code  = true
  kind  = "item-covered-sample"
  note  = "existing code, issues, or PRs cover the work"
  state = "covered"
  same  = [50329, 50385, 50428, 50471, 50482, 50483, 50551, 50660, 50679, 50218]
  same_paths = [
    "internal/service/cloudwatchomni/access_grant.go",
    "internal/service/cloudwatchomni/access_grant_list.go",
    "internal/service/cloudwatchomni/access_profile.go",
    "internal/service/cloudwatchomni/dashboard.go",
    "internal/service/cloudwatchomni/domain.go",
    "internal/service/cloudwatchomni/domain_list.go",
    "internal/service/cloudwatchomni/organization_domain.go",
    "internal/service/cloudwatchomni/organization_domain_list.go",
    "internal/service/cloudwatchomni/space.go",
    "internal/service/cloudwatchomni/view.go",
  ]
}

case "item-covered-sample-devopsagent-2026-03-30" {
  item  = "aws-sdk-go-v2/devopsagent/release-2026-03-30"
  as_of = "2026-11-13T19:00:00Z"
  code  = true
  kind  = "item-covered-sample"
  note  = "private connection (open PR #48148, not cited) and operator app have no code; first labeled covered"
  state = "partial"
  same  = [47795, 50711, 50712]
  same_paths = [
    "internal/service/devopsagent/agent_space.go",
  ]
}

case "item-covered-sample-devopsagent-2026-06-08" {
  item  = "aws-sdk-go-v2/devopsagent/release-2026-06-08"
  as_of = "2026-11-13T19:00:00Z"
  code  = true
  kind  = "item-covered-sample"
  note  = "existing code, issues, or PRs cover the work"
  state = "covered"
  same_paths = [
    "internal/service/devopsagent/asset.go",
    "internal/service/devopsagent/asset_file.go",
    "internal/service/devopsagent/asset_file_list.go",
  ]
}

case "item-covered-sample-ec2-2026-02-10" {
  item  = "aws-sdk-go-v2/ec2/release-2026-02-10"
  as_of = "2026-11-13T19:00:00Z"
  code  = true
  kind  = "item-covered-sample"
  note  = "aws_instance secondary_interfaces has no code or issue; the launch template has it"
  state = "partial"
  different = [46552]
  same_paths = [
    "internal/service/ec2/ec2_secondary_network.go",
    "internal/service/ec2/ec2_secondary_subnet.go",
  ]
}

case "item-covered-sample-ec2-2026-07-29" {
  item  = "aws-sdk-go-v2/ec2/release-2026-07-29"
  as_of = "2026-11-13T19:00:00Z"
  code  = true
  kind  = "item-covered-sample"
  note  = "existing code, issues, or PRs cover the work"
  state = "covered"
  same_paths = [
    "internal/service/ec2/transitgateway_policy_table_entry.go",
  ]
}

case "item-covered-sample-ecs-2026-10-20" {
  item  = "aws-sdk-go-v2/ecs/release-2026-10-20"
  as_of = "2026-11-13T19:00:00Z"
  code  = true
  kind  = "item-covered-sample"
  note  = "existing code, issues, or PRs cover the work"
  state = "covered"
  same  = [50894, 50895, 50573, 50677, 50896]
}

case "item-covered-sample-kafka-2026-02-10-2" {
  item  = "aws-sdk-go-v2/kafka/release-2026-02-10.2"
  as_of = "2026-11-13T19:00:00Z"
  code  = true
  kind  = "item-covered-sample"
  note  = "existing code, issues, or PRs cover the work"
  state = "covered"
  same  = [46640]
  different = [50323, 50388, 50721, 50723]
  same_paths = [
    "internal/service/kafka/topic.go",
  ]
}

case "item-covered-sample-kafka-2026-07-30" {
  item  = "aws-sdk-go-v2/kafka/release-2026-07-30"
  as_of = "2026-11-13T19:00:00Z"
  code  = true
  kind  = "item-covered-sample"
  note  = "existing code, issues, or PRs cover the work"
  state = "covered"
  same  = [49266]
  same_paths = [
    "internal/service/kafka/channel.go",
  ]
}

case "item-covered-sample-lambda-2026-08-20" {
  item  = "aws-sdk-go-v2/lambda/release-2026-08-20"
  as_of = "2026-11-13T19:00:00Z"
  code  = true
  kind  = "item-covered-sample"
  note  = "existing code, issues, or PRs cover the work"
  state = "covered"
  same  = [49866]
  same_paths = [
    "internal/service/lambda/resource_policy.go",
  ]
}

case "item-covered-sample-lambdaweb-2026-10-01" {
  item  = "aws-sdk-go-v2/lambdaweb/release-2026-10-01"
  as_of = "2026-11-13T19:00:00Z"
  code  = true
  kind  = "item-covered-sample"
  note  = "existing code, issues, or PRs cover the work"
  state = "covered"
  same  = [50341, 50452]
  different = [50719]
  same_paths = [
    "internal/service/lambdaweb/function.go",
    "internal/service/lambdaweb/function_endpoint.go",
    "internal/service/lambdaweb/function_revision.go",
    "internal/service/lambdaweb/generate.go",
    "internal/service/lambdaweb/resource_policy.go",
    "internal/service/lambdaweb/tags_gen.go",
  ]
}

case "item-covered-sample-networkfirewall-2026-11-10" {
  item  = "aws-sdk-go-v2/networkfirewall/release-2026-11-10"
  as_of = "2026-11-13T19:00:00Z"
  code  = true
  kind  = "item-covered-sample"
  note  = "rule group firewall_rule_group and ips_rule_group have no code; the policy v2 tracker #50901 does not mention them; first labeled covered"
  state = "partial"
  same  = [50901]
}

case "item-covered-sample-observabilityadmin-2025-12-02" {
  item  = "aws-sdk-go-v2/observabilityadmin/release-2025-12-02"
  as_of = "2026-11-13T19:00:00Z"
  code  = true
  kind  = "item-covered-sample"
  note  = "existing code, issues, or PRs cover the work"
  state = "covered"
  same  = [48137, 48190]
  different = [48286]
  same_paths = [
    "internal/service/observabilityadmin/s3_table_integration.go",
    "internal/service/observabilityadmin/telemetry_pipeline.go",
  ]
}

case "item-covered-sample-observabilityadmin-2026-09-22" {
  item  = "aws-sdk-go-v2/observabilityadmin/release-2026-09-22"
  as_of = "2026-11-13T19:00:00Z"
  code  = true
  kind  = "item-covered-sample"
  note  = "centralization rule context graph configuration has no code or issue"
  state = "partial"
  same  = [50505, 50529]
  same_paths = [
    "internal/service/observabilityadmin/dataset_integration.go",
  ]
}

case "item-covered-sample-odb-2026-06-09" {
  item  = "aws-sdk-go-v2/odb/release-2026-06-09"
  as_of = "2026-11-13T19:00:00Z"
  code  = true
  kind  = "item-covered-sample"
  note  = "autonomous database backup has no code or issue"
  state = "partial"
  same  = [48991]
  same_paths = [
    "internal/service/odb/autonomous_database.go",
  ]
}

case "item-covered-sample-opensearch-2026-03-31" {
  item  = "aws-sdk-go-v2/opensearch/release-2026-03-31"
  as_of = "2026-11-13T19:00:00Z"
  code  = true
  kind  = "item-covered-sample"
  note  = "existing code, issues, or PRs cover the work"
  state = "covered"
  same  = [49494, 50606]
}

case "item-covered-sample-pricingplanmanager-2026-07-30" {
  item  = "aws-sdk-go-v2/pricingplanmanager/release-2026-07-30"
  as_of = "2026-11-13T19:00:00Z"
  code  = true
  kind  = "item-covered-sample"
  note  = "existing code, issues, or PRs cover the work"
  state = "covered"
  same  = [45450, 49235]
  different = [50498]
  same_paths = [
    "internal/service/pricingplanmanager/subscription.go",
  ]
}

case "item-covered-sample-s3-2026-01-28" {
  item  = "aws-sdk-go-v2/s3/release-2026-01-28"
  as_of = "2026-11-13T19:00:00Z"
  code  = true
  kind  = "item-covered-sample"
  note  = "existing code, issues, or PRs cover the work"
  state = "covered"
  same  = [46261, 46264, 46286]
}

case "item-covered-sample-s3files-2026-04-07" {
  item  = "aws-sdk-go-v2/s3files/release-2026-04-07"
  as_of = "2026-11-13T19:00:00Z"
  code  = true
  kind  = "item-covered-sample"
  note  = "existing code, issues, or PRs cover the work"
  state = "covered"
  same  = [47333, 47334, 47336, 47337]
  different = [47430, 47510, 47575, 47588, 47664, 47753, 47760, 48725, 49204, 50920]
  same_paths = [
    "internal/service/s3files/access_point.go",
    "internal/service/s3files/file_system.go",
    "internal/service/s3files/file_system_list.go",
    "internal/service/s3files/file_system_policy.go",
    "internal/service/s3files/file_systems_data_source.go",
    "internal/service/s3files/mount_target.go",
    "internal/service/s3files/sweep.go",
    "internal/service/s3files/synchronization_configuration.go",
  ]
}

case "item-covered-sample-sagemaker-2025-12-02" {
  item  = "aws-sdk-go-v2/sagemaker/release-2025-12-02"
  as_of = "2026-11-13T19:00:00Z"
  code  = true
  kind  = "item-covered-sample"
  note  = "existing code, issues, or PRs cover the work"
  state = "covered"
  same_paths = [
    "internal/service/sagemaker/mlflow_app.go",
  ]
}

case "item-covered-sample-ssoadmin-2026-02-03" {
  item  = "aws-sdk-go-v2/ssoadmin/release-2026-02-03"
  as_of = "2026-11-13T19:00:00Z"
  code  = true
  kind  = "item-covered-sample"
  note  = "existing code, issues, or PRs cover the work"
  state = "covered"
  same  = [48126, 48397, 48398]
  same_paths = [
    "internal/service/ssoadmin/region.go",
  ]
}

case "item-covered-sample-uxc-2026-03-25" {
  item  = "aws-sdk-go-v2/uxc/release-2026-03-25"
  as_of = "2026-11-13T19:00:00Z"
  code  = true
  kind  = "item-covered-sample"
  note  = "existing code, issues, or PRs cover the work"
  state = "covered"
  same_paths = [
    "internal/service/uxc/account_customizations.go",
  ]
}

case "item-partial-networkmanager-2026-10-22" {
  item  = "aws-sdk-go-v2/networkmanager/release-2026-10-22"
  as_of = "2026-11-13T19:00:00Z"
  code  = true
  kind  = "item-partial"
  note  = "tracker #50613 covers the launch, including nat_gateway_rules"
  state = "covered"
  same  = [50613, 50689, 50690, 50700]
  different = [50669, 50672, 50683]
}

case "item-partial-s3-2026-09-08" {
  item  = "aws-sdk-go-v2/s3/release-2026-09-08"
  as_of = "2026-11-13T19:00:00Z"
  code  = true
  kind  = "item-partial"
  note  = "tracker #49909 covers object lock variable retention"
  state = "covered"
  same  = [49909, 50220]
}

case "item-partial-dynamodb-2026-08-04" {
  item  = "aws-sdk-go-v2/dynamodb/release-2026-08-04"
  as_of = "2026-11-13T19:00:00Z"
  code  = true
  kind  = "item-partial"
  note  = "tracker #49350 covers vector search, including vector index arguments"
  state = "covered"
  same  = [49350, 49375]
}

case "item-partial-ec2-2026-08-04" {
  item  = "aws-sdk-go-v2/ec2/release-2026-08-04"
  as_of = "2026-11-13T19:00:00Z"
  code  = true
  kind  = "item-partial"
  note  = "application status check suppression has no code or issue"
  state = "partial"
  same  = [49487, 49490, 49391]
}

case "item-partial-configservice-2026-07-07" {
  item  = "aws-sdk-go-v2/configservice/release-2026-07-07"
  as_of = "2026-11-13T19:00:00Z"
  code  = true
  kind  = "item-partial"
  note  = "configuration recorder connector_arn and scope_configuration have no code or issue"
  state = "partial"
  same  = [49559]
  same_paths = [
    "internal/service/configservice/connector.go",
  ]
}

case "item-partial-securityhub-2026-07-07" {
  item  = "aws-sdk-go-v2/securityhub/release-2026-07-07"
  as_of = "2026-11-13T19:00:00Z"
  code  = true
  kind  = "item-partial"
  note  = "resource_owner_* arguments have no code or issue"
  state = "partial"
  same  = [49503, 49568, 49560]
  different = [49559]
  same_paths = [
    "internal/service/securityhub/feature_v2.go",
  ]
}

case "item-partial-cloudformation-2026-06-30" {
  item  = "aws-sdk-go-v2/cloudformation/release-2026-06-30"
  as_of = "2026-11-13T19:00:00Z"
  code  = true
  kind  = "item-partial"
  note  = "tracker #48728 covers Express mode, including deployment_config"
  state = "covered"
  same  = [48728]
}

case "item-partial-eks-2026-06-30" {
  item  = "aws-sdk-go-v2/eks/release-2026-06-30"
  as_of = "2026-11-13T19:00:00Z"
  code  = true
  kind  = "item-partial"
  note  = "tracker #48751 covers version rollback"
  state = "covered"
  same  = [48751]
}

case "item-partial-cloudwatchlogs-2026-06-22" {
  item  = "aws-sdk-go-v2/cloudwatchlogs/release-2026-06-22"
  as_of = "2026-11-13T19:00:00Z"
  code  = true
  kind  = "item-partial"
  note  = "tracker #48547 covers syslog ingestion"
  state = "covered"
  same  = [48547]
}

case "item-partial-bedrockagentcorecontrol-2026-06-17" {
  item  = "aws-sdk-go-v2/bedrockagentcorecontrol/release-2026-06-17"
  as_of = "2026-11-13T19:00:00Z"
  code  = true
  kind  = "item-partial"
  note  = "four new arguments have no code; open PRs #48711, #48677, #48875 cover three, not cited"
  state = "partial"
  same  = [48791, 48496, 48654, 48661, 48710, 48711, 48818, 48875, 48903]
  different = [48699]
}

case "item-partial-s3-2026-06-16" {
  item  = "aws-sdk-go-v2/s3/release-2026-06-16"
  as_of = "2026-11-13T19:00:00Z"
  code  = true
  kind  = "item-partial"
  note  = "tracker #48444 covers object annotations"
  state = "covered"
  same  = [48444, 50121]
}

case "item-partial-sesv2-2026-05-29" {
  item  = "aws-sdk-go-v2/sesv2/release-2026-05-29"
  as_of = "2026-11-13T19:00:00Z"
  code  = true
  kind  = "item-partial"
  note  = "#49357 and #50861 cover tenant suppression attributes"
  state = "covered"
  same  = [49357, 50861, 48236]
}

case "item-partial-rds-2026-03-24" {
  item  = "aws-sdk-go-v2/rds/release-2026-03-24"
  as_of = "2026-11-13T19:00:00Z"
  code  = true
  kind  = "item-partial"
  note  = "tracker #47117 covers express configuration"
  state = "covered"
  same  = [47117]
}

case "item-partial-bedrockagentcorecontrol-2025-12-02" {
  item  = "aws-sdk-go-v2/bedrockagentcorecontrol/release-2025-12-02"
  as_of = "2026-11-13T19:00:00Z"
  code  = true
  kind  = "item-partial"
  note  = "episodic memory strategy arguments have no code or issue"
  state = "partial"
  same  = [47108, 47209, 47957, 47964, 47971, 45599, 47589]
  same_paths = [
    "internal/service/bedrockagentcore/evaluator.go",
    "internal/service/bedrockagentcore/gateway.go",
    "internal/service/bedrockagentcore/online_evaluation_config.go",
    "internal/service/bedrockagentcore/policy.go",
    "internal/service/bedrockagentcore/policy_engine.go",
    "internal/service/bedrockagentcore/resource_policy.go",
  ]
}

case "item-partial-lambda-2025-12-02" {
  item  = "aws-sdk-go-v2/lambda/release-2025-12-02"
  as_of = "2026-11-13T19:00:00Z"
  code  = true
  kind  = "item-partial"
  note  = "tracker #45354 covers durable functions; durable_execution_name is an invoke parameter"
  state = "covered"
  same  = [45354, 45359]
  same_paths = [
    "internal/service/lambda/function.go",
  ]
  different = [45800]
}

# --- blog
# Blog posts linked from provider issues created since 2025-11-18, each mapped by hand to the SDK item the issue
# tracks. Any listed post in the top 5 of the item's blog posts (brief's list) passes.

case "blog-eks-2025-12-01" {
  item = "aws-sdk-go-v2/eks/release-2025-12-01"
  kind = "blog"
  note = "#45324"
  blog = ["https://aws.amazon.com/blogs/aws/announcing-amazon-eks-capabilities-for-workload-orchestration-and-cloud-resource-management/"]
}

case "blog-lambda-2025-12-01" {
  item = "aws-sdk-go-v2/lambda/release-2025-12-01"
  kind = "blog"
  note = "#45332"
  blog = ["https://aws.amazon.com/blogs/aws/introducing-aws-lambda-managed-instances-serverless-simplicity-with-ec2-flexibility/"]
}

case "blog-bedrockagentcorecontrol-2025-12-02" {
  item = "aws-sdk-go-v2/bedrockagentcorecontrol/release-2025-12-02"
  kind = "blog"
  note = "#46878"
  blog = ["https://aws.amazon.com/blogs/machine-learning/streamline-ai-agent-tool-interactions-connect-api-gateway-to-agentcore-gateway-with-mcp/"]
}

case "blog-fsx-2025-12-02" {
  item = "aws-sdk-go-v2/fsx/release-2025-12-02"
  kind = "blog"
  note = "#45362"
  blog = ["https://aws.amazon.com/blogs/aws/amazon-fsx-for-netapp-ontap-now-integrates-with-amazon-s3-for-seamless-data-access/"]
}

case "blog-lambda-2025-12-02" {
  item = "aws-sdk-go-v2/lambda/release-2025-12-02"
  kind = "blog"
  note = "#45354"
  blog = ["https://aws.amazon.com/blogs/aws/build-multi-step-applications-and-ai-workflows-with-aws-lambda-durable-functions/"]
}

case "blog-s3control-2025-12-02" {
  item = "aws-sdk-go-v2/s3control/release-2025-12-02"
  kind = "blog"
  note = "#46851"
  blog = ["https://aws.amazon.com/blogs/aws/amazon-s3-storage-lens-adds-performance-metrics-support-for-billions-of-prefixes-and-export-to-s3-tables/"]
}

case "blog-s3tables-2025-12-02" {
  item = "aws-sdk-go-v2/s3tables/release-2025-12-02"
  kind = "blog"
  note = "#45353, #45361, #45586"
  blog = ["https://aws.amazon.com/blogs/aws/announcing-replication-support-and-intelligent-tiering-for-amazon-s3-tables/"]
}

case "blog-sagemaker-2025-12-02" {
  item = "aws-sdk-go-v2/sagemaker/release-2025-12-02"
  kind = "blog"
  note = "#46194"
  blog = ["https://aws.amazon.com/blogs/machine-learning/scaling-mlflow-for-enterprise-ai-whats-new-in-sagemaker-ai-with-mlflow/"]
}

case "blog-s3-2026-01-28" {
  item = "aws-sdk-go-v2/s3/release-2026-01-28"
  kind = "blog"
  note = "#46261"
  blog = ["https://aws.amazon.com/blogs/aws/aws-weekly-roundup-amazon-bedrock-agent-workflows-amazon-sagemaker-private-connectivity-and-more-february-2-2026/"]
}

case "blog-cloudfront-2026-02-02" {
  item = "aws-sdk-go-v2/cloudfront/release-2026-02-02"
  kind = "blog"
  note = "#46360"
  blog = ["https://aws.amazon.com/blogs/networking-and-content-delivery/amazon-cloudfront-now-supports-mtls-authentication-to-origins/"]
}

case "blog-s3-2026-03-12" {
  item = "aws-sdk-go-v2/s3/release-2026-03-12"
  kind = "blog"
  note = "#46902, #46962, #47073"
  blog = ["https://aws.amazon.com/blogs/aws/introducing-account-regional-namespaces-for-amazon-s3-general-purpose-buckets/"]
}

case "blog-rds-2026-03-24" {
  item = "aws-sdk-go-v2/rds/release-2026-03-24"
  kind = "blog"
  note = "#47117"
  blog = ["https://aws.amazon.com/blogs/aws/announcing-amazon-aurora-postgresql-serverless-database-creation-in-seconds/"]
}

case "blog-ecs-2026-04-01" {
  item = "aws-sdk-go-v2/ecs/release-2026-04-01"
  kind = "blog"
  note = "#47200"
  blog = ["https://aws.amazon.com/blogs/aws/announcing-managed-daemon-support-for-amazon-ecs-managed-instances/"]
}

case "blog-s3files-2026-04-07" {
  item = "aws-sdk-go-v2/s3files/release-2026-04-07"
  kind = "blog"
  note = "#47324"
  blog = ["https://aws.amazon.com/blogs/aws/launching-s3-files-making-s3-buckets-accessible-as-file-systems/"]
}

case "blog-bedrockagentcorecontrol-2026-04-09" {
  item = "aws-sdk-go-v2/bedrockagentcorecontrol/release-2026-04-09"
  kind = "blog"
  note = "#48501"
  blog = ["https://aws.amazon.com/blogs/machine-learning/the-future-of-managing-agents-at-scale-aws-agent-registry-now-in-preview/"]
}

case "blog-interconnect-2026-04-13" {
  item = "aws-sdk-go-v2/interconnect/release-2026-04-13"
  kind = "blog"
  note = "#47458"
  blog = ["https://aws.amazon.com/blogs/aws/aws-interconnect-is-now-generally-available-with-a-new-option-to-simplify-last-mile-connectivity/"]
}

case "blog-ec2-2026-04-20" {
  item = "aws-sdk-go-v2/ec2/release-2026-04-20"
  kind = "blog"
  note = "#47627"
  blog = ["https://aws.amazon.com/blogs/networking-and-content-delivery/introducing-aws-client-vpn-native-aws-transit-gateway-attachment/"]
}

case "blog-cloudfront-2026-04-29" {
  item = "aws-sdk-go-v2/cloudfront/release-2026-04-29"
  kind = "blog"
  note = "#47865"
  blog = ["https://aws.amazon.com/blogs/networking-and-content-delivery/manage-caches-with-precision-using-amazon-cloudfront-invalidation-by-cache-tag/"]
}

case "blog-sagemaker-2026-04-30" {
  item = "aws-sdk-go-v2/sagemaker/release-2026-04-30"
  kind = "blog"
  note = "#48469"
  blog = ["https://aws.amazon.com/blogs/machine-learning/capacity-aware-inference-automatic-instance-fallback-for-sagemaker-ai-endpoints/"]
}

case "blog-opensearchserverless-2026-05-28" {
  item = "aws-sdk-go-v2/opensearchserverless/release-2026-05-28"
  kind = "blog"
  note = "#48113, #48183"
  blog = [
    "https://aws.amazon.com/blogs/aws/introducing-the-next-generation-of-amazon-opensearch-serverless-for-building-your-agentic-ai-applications/",
    "https://aws.amazon.com/blogs/big-data/the-next-generation-of-amazon-opensearch-serverless-built-from-the-ground-up-for-agents/",
  ]
}

case "blog-resiliencehubv2-2026-05-28" {
  item = "aws-sdk-go-v2/resiliencehubv2/release-2026-05-28"
  kind = "blog"
  note = "#48122"
  blog = ["https://aws.amazon.com/blogs/aws/introducing-the-next-generation-of-aws-resilience-hub-for-generative-ai-based-sre-resilience-journey/"]
}

case "blog-elasticache-2026-06-02" {
  item = "aws-sdk-go-v2/elasticache/release-2026-06-02"
  kind = "blog"
  note = "#48228"
  blog = ["https://aws.amazon.com/blogs/database/announcing-durability-for-amazon-elasticache/"]
}

case "blog-wafv2-2026-06-15" {
  item = "aws-sdk-go-v2/wafv2/release-2026-06-15"
  kind = "blog"
  note = "#48418"
  blog = ["https://aws.amazon.com/blogs/aws/aws-waf-adds-ai-traffic-monetization-capability-to-help-content-owners-charge-ai-bots-for-content-access/"]
}

case "blog-s3-2026-06-16" {
  item = "aws-sdk-go-v2/s3/release-2026-06-16"
  kind = "blog"
  note = "#48444"
  blog = ["https://aws.amazon.com/blogs/aws/amazon-s3-annotations-attach-rich-queryable-context-directly-to-your-objects/"]
}

case "blog-mq-2026-06-17" {
  item = "aws-sdk-go-v2/mq/release-2026-06-17"
  kind = "blog"
  note = "#48487"
  blog = ["https://aws.amazon.com/blogs/big-data/introducing-private-networking-for-amazon-mq-for-rabbitmq/"]
}

case "blog-ecs-2026-06-18" {
  item = "aws-sdk-go-v2/ecs/release-2026-06-18"
  kind = "blog"
  note = "#48669"
  blog = ["https://aws.amazon.com/blogs/aws/amazon-ecs-introduces-new-high-resolution-metrics-for-faster-service-auto-scaling/"]
}

case "blog-eks-2026-06-18" {
  item = "aws-sdk-go-v2/eks/release-2026-06-18"
  kind = "blog"
  note = "#48470"
  blog = ["https://aws.amazon.com/blogs/containers/amazon-eks-now-supports-control-plane-egress-through-your-vpc/"]
}

case "blog-lambdamicrovms-2026-06-22" {
  item = "aws-sdk-go-v2/lambdamicrovms/release-2026-06-22"
  kind = "blog"
  note = "#48526"
  blog = ["https://aws.amazon.com/blogs/aws/run-isolated-sandboxes-with-full-lifecycle-control-aws-lambda-introduces-microvms/"]
}

case "blog-acm-2026-06-30" {
  item = "aws-sdk-go-v2/acm/release-2026-06-30"
  kind = "blog"
  note = "#48727"
  blog = ["https://aws.amazon.com/blogs/aws/automate-public-tls-certificate-issuance-with-acme-support-in-aws-certificate-manager/"]
}

case "blog-cloudformation-2026-06-30" {
  item = "aws-sdk-go-v2/cloudformation/release-2026-06-30"
  kind = "blog"
  note = "#48728"
  blog = ["https://aws.amazon.com/blogs/aws/accelerate-your-infrastructure-deployments-by-up-to-4x-with-aws-cloudformation-express-mode/"]
}

case "blog-eks-2026-06-30" {
  item = "aws-sdk-go-v2/eks/release-2026-06-30"
  kind = "blog"
  note = "#48751"
  blog = [
    "https://aws.amazon.com/blogs/aws/upgrade-amazon-eks-clusters-with-confidence-using-kubernetes-version-rollbacks/",
    "https://aws.amazon.com/blogs/containers/announcing-amazon-eks-rollback-for-safe-and-reliable-management-of-cluster-upgrades/",
  ]
}

case "blog-elasticloadbalancingv2-2026-07-15" {
  item = "aws-sdk-go-v2/elasticloadbalancingv2/release-2026-07-15"
  kind = "blog"
  note = "#49368"
  blog = ["https://aws.amazon.com/blogs/networking-and-content-delivery/consolidate-dual-stack-architectures-with-listener-rules-for-network-load-balancer/"]
}

case "blog-dynamodb-2026-08-04" {
  item = "aws-sdk-go-v2/dynamodb/release-2026-08-04"
  kind = "blog"
  note = "#49350"
  blog = ["https://aws.amazon.com/blogs/aws/amazon-dynamodb-now-supports-real-time-vector-search-at-any-scale/"]
}

case "blog-bedrockagentcorecontrol-2026-08-06" {
  item = "aws-sdk-go-v2/bedrockagentcorecontrol/release-2026-08-06"
  kind = "blog"
  note = "#49344, #49352"
  blog = [
    "https://aws.amazon.com/blogs/machine-learning/configure-rate-limits-for-ai-traffic-on-agentcore-gateway/",
    "https://aws.amazon.com/blogs/aws/runtime-instances-persistent-compute-for-production-ai-agents-on-amazon-bedrock-agentcore/",
  ]
}

case "blog-s3-2026-08-06" {
  item = "aws-sdk-go-v2/s3/release-2026-08-06"
  kind = "blog"
  note = "#49351"
  blog = ["https://aws.amazon.com/blogs/storage/access-amazon-s3-backup-data-directly-using-s3-access-points-in-aws-backup/"]
}

case "blog-eks-2026-08-11" {
  item = "aws-sdk-go-v2/eks/release-2026-08-11"
  kind = "blog"
  note = "#49411"
  blog = ["https://aws.amazon.com/blogs/containers/introducing-advanced-kubernetes-control-plane-configuration-in-amazon-eks/"]
}

case "blog-odb-2026-08-12" {
  item = "aws-sdk-go-v2/odb/release-2026-08-12"
  kind = "blog"
  note = "#49464"
  blog = ["https://aws.amazon.com/blogs/database/introducing-oracle-exadata-on-exascale-for-oracle-ai-databaseaws/"]
}

case "blog-redshift-2026-08-19" {
  item = "aws-sdk-go-v2/redshift/release-2026-08-19"
  kind = "blog"
  note = "#49640"
  blog = ["https://aws.amazon.com/blogs/big-data/long-term-system-tables-retention-in-amazon-redshift-with-amazon-s3-tables/"]
}

case "blog-kinesis-2026-08-31" {
  item = "aws-sdk-go-v2/kinesis/release-2026-08-31"
  kind = "blog"
  note = "#49792"
  blog = ["https://aws.amazon.com/blogs/big-data/deliver-real-time-data-to-streaming-tables-for-apache-iceberg-with-amazon-kinesis-data-streams/"]
}

case "blog-bedrockagentcorecontrol-2026-09-03" {
  item = "aws-sdk-go-v2/bedrockagentcorecontrol/release-2026-09-03"
  kind = "blog"
  note = "#49882"
  blog = ["https://aws.amazon.com/blogs/machine-learning/manage-end-user-oauth-consent-for-ai-agents-with-amazon-bedrock-agentcore/"]
}

case "blog-s3-2026-09-08" {
  item = "aws-sdk-go-v2/s3/release-2026-09-08"
  kind = "blog"
  note = "#49909"
  blog = ["https://aws.amazon.com/blogs/storage/flexibly-control-amazon-s3-object-lock-retention-based-on-real-business-events/"]
}

case "blog-bedrockagentcorecontrol-2026-09-15" {
  item = "aws-sdk-go-v2/bedrockagentcorecontrol/release-2026-09-15"
  kind = "blog"
  note = "#50244"
  blog = ["https://aws.amazon.com/blogs/machine-learning/the-new-agentcore-runtime-elastic-optimized-and-consistently-fast-starts/"]
}

case "blog-elasticbeanstalk-2026-09-16" {
  item = "aws-sdk-go-v2/elasticbeanstalk/release-2026-09-16"
  kind = "blog"
  note = "#50064"
  blog = ["https://aws.amazon.com/blogs/aws/aws-elastic-beanstalk-introduces-cluster-mode/"]
}

case "blog-cloudwatchomni-2026-09-22" {
  item = "aws-sdk-go-v2/cloudwatchomni/release-2026-09-22"
  kind = "blog"
  note = "#50218"
  blog = [
    "https://aws.amazon.com/blogs/aws/introducing-amazon-cloudwatch-omni-ai-powered-observability-for-generative-ai-and-agentic-workloads/",
    "https://aws.amazon.com/blogs/aws/introducing-amazon-cloudwatch-omni-collaborative-ai-powered-observability-for-your-applications/",
  ]
}

case "blog-eventbridgev2-2026-09-24" {
  item = "aws-sdk-go-v2/eventbridgev2/release-2026-09-24"
  kind = "blog"
  note = "#50154"
  blog = ["https://aws.amazon.com/blogs/aws/introducing-enhanced-custom-event-buses-in-amazon-eventbridge-for-enterprise-scale-event-driven-applications/"]
}

case "blog-lambdaweb-2026-10-12" {
  item = "aws-sdk-go-v2/lambdaweb/release-2026-10-12"
  kind = "blog"
  note = "#50429"
  blog = ["https://aws.amazon.com/blogs/aws/announcing-aws-lambda-web-functions-for-fast-consistent-web-app-performance-anywhere/"]
}

case "blog-ecs-2026-10-20" {
  item = "aws-sdk-go-v2/ecs/release-2026-10-20"
  kind = "blog"
  note = "#50573"
  blog = ["https://aws.amazon.com/blogs/aws/introducing-amazon-ecs-gateway-a-fully-managed-ingress-solution-for-ecs-services/"]
}

case "blog-kinesisanalyticsv2-2026-11-11" {
  item = "aws-sdk-go-v2/kinesisanalyticsv2/release-2026-11-11"
  kind = "blog"
  note = "#51073"
  blog = ["https://aws.amazon.com/blogs/big-data/introducing-enhanced-snapshots-for-amazon-managed-service-for-apache-flink/"]
}

case "blog-dsql-2026-11-16" {
  item = "aws-sdk-go-v2/dsql/release-2026-11-16"
  kind = "blog"
  note = "#51095"
  blog = ["https://aws.amazon.com/blogs/database/use-query-statistics-to-find-slow-and-expensive-queries-in-aurora-dsql/"]
}

case "blog-lambda-2026-11-16" {
  item = "aws-sdk-go-v2/lambda/release-2026-11-16"
  kind = "blog"
  note = "#51046"
  blog = ["https://aws.amazon.com/blogs/aws/aws-lambda-now-supports-gpu-instances-for-event-functions-on-lambda-managed-instances/"]
}

case "blog-sagemaker-2026-11-16" {
  item = "aws-sdk-go-v2/sagemaker/release-2026-11-16"
  kind = "blog"
  note = "#51090"
  blog = ["https://aws.amazon.com/blogs/machine-learning/how-salesforce-is-advancing-global-scale-model-serving-with-cross-region-inference-routing-in-amazon-sagemaker-ai/"]
}

# --- posts
# Blog posts from 2026-11-02 to 2026-11-19 decided in the 2026-11-19 coverage pass: the 104 launch posts and the 8
# other posts with a decision or an issue. Post cases reconcile the post as `avizor posts` does and ignore decisions.
# A post with work must be significant. Post cases need pulled blog posts and announcements: run
# `avizor pull aws-blogs --since 365d` and `avizor pull aws-whats-new --since 365d` first.

case "post-aws-weekly-roundup-gpt-6-1-terra-on-bedrock-step-f" {
  post  = "https://aws.amazon.com/blogs/aws/aws-weekly-roundup-gpt-6-1-terra-on-bedrock-step-functions-as-agent-tools-and-more-november-2-2026/"
  kind  = "post-no-work"
  note  = "weekly roundup; its launches have their own posts and announcements"
  work  = false
}

case "post-introducing-queue-consumption-mode-for-apache-kafk" {
  post  = "https://aws.amazon.com/blogs/compute/introducing-queue-consumption-mode-for-apache-kafka-in-aws-lambda/"
  kind  = "post-covered"
  note  = "labeled in the coverage pass"
  as_of = "2026-11-19"
  state = "covered"
  work  = true
  expect {
    number = 50775
  }
  same  = [50778]
}

case "post-aws-contributes-a-large-transaction-binary-log-opt" {
  post  = "https://aws.amazon.com/blogs/database/aws-contributes-a-large-transaction-binary-log-optimization-to-mysql/"
  kind  = "post-no-work"
  note  = "customer story or how-to guide"
  work  = false
}

case "post-inside-a-successful-aws-european-sovereign-cloud-e" {
  post  = "https://aws.amazon.com/blogs/security/inside-a-successful-aws-european-sovereign-cloud-exercise-to-demonstrate-independent-operation/"
  kind  = "post-no-work"
  note  = "customer story or how-to guide"
  work  = false
}

case "post-deep-dive-auto-scaling-response-times-on-amazon-ec" {
  post  = "https://aws.amazon.com/blogs/containers/deep-dive-auto-scaling-response-times-on-amazon-ecs/"
  kind  = "post-no-work"
  note  = "customer story or how-to guide"
  work  = false
}

case "post-introducing-india-based-inference-for-kimi-k3-on-a" {
  post  = "https://aws.amazon.com/blogs/machine-learning/introducing-india-based-inference-for-kimi-k3-on-amazon-bedrock/"
  kind  = "post-no-work"
  note  = "model availability"
  work  = false
}

case "post-find-your-logging-gaps-before-they-become-complian" {
  post  = "https://aws.amazon.com/blogs/mt/find-your-logging-gaps-before-they-become-compliance-gaps-with-cloudtrail-data-event-coverage/"
  kind  = "post-no-work"
  note  = "no new resource or argument"
  work  = false
}

case "post-tag-editor-moves-to-aws-resource-explorer-bulk-tag" {
  post  = "https://aws.amazon.com/blogs/mt/tag-editor-moves-to-aws-resource-explorer-bulk-tag-resources-across-regions/"
  kind  = "post-no-work"
  note  = "no new resource or argument"
  work  = false
}

case "post-introducing-change-based-throttling-for-amazon-rou" {
  post  = "https://aws.amazon.com/blogs/networking-and-content-delivery/introducing-change-based-throttling-for-amazon-route-53-apis/"
  kind  = "post-no-work"
  note  = "no new resource or argument"
  work  = false
}

case "post-amazon-s3-tables-now-offer-managed-migration-of-ap" {
  post  = "https://aws.amazon.com/blogs/aws/amazon-s3-tables-now-offer-managed-migration-of-apache-hive-tables-to-apache-iceberg/"
  kind  = "post-no-work"
  note  = "no new resource or argument"
  work  = false
}

case "post-plan-and-measure-ec2-flexibility-with-amazon-ec2-c" {
  post  = "https://aws.amazon.com/blogs/compute/plan-and-measure-ec2-flexibility-with-amazon-ec2-capacity-manager/"
  kind  = "post-no-work"
  note  = "supportability scores and metrics are read-only; no resource"
  work  = false
}

case "post-introducing-bulk-put-items-for-amazon-dynamodb-loa" {
  post  = "https://aws.amazon.com/blogs/database/introducing-bulk-put-items-for-amazon-dynamodb-load-data-from-amazon-s3-into-existing-tables-at-scale/"
  kind  = "post-no-work"
  note  = "no new resource or argument"
  work  = false
}

case "post-introducing-presigned-url-downloads-in-the-aws-sdk" {
  post  = "https://aws.amazon.com/blogs/developer/introducing-presigned-url-downloads-in-the-aws-sdk-for-java-2-x/"
  kind  = "post-no-work"
  note  = "SDK or developer tooling"
  work  = false
}

case "post-introducing-device-posture-assessment-for-aws-clie" {
  post  = "https://aws.amazon.com/blogs/networking-and-content-delivery/introducing-device-posture-assessment-for-aws-client-vpn/"
  kind  = "post-gap"
  note  = "filed afterward as #51133, #51134"
  as_of = "2026-11-19T19:15:00Z"
  state = "open"
  work  = true
}

case "post-introducing-tunnel-endpoints-for-private-access-to" {
  post  = "https://aws.amazon.com/blogs/networking-and-content-delivery/introducing-tunnel-endpoints-for-private-access-to-network-segments/"
  kind  = "post-covered"
  note  = "decided as a duplicate; nothing links the post or its announcement"
  as_of = "2026-11-19"
  state = "covered"
  work  = true
  expect {
    number = 50087
  }
  expect {
    number = 50553
  }
}

case "post-aws-systems-manager-for-sap-now-supports-sap-hana" {
  post  = "https://aws.amazon.com/blogs/awsforsap/aws-systems-manager-for-sap-now-supports-sap-hana-scale-out-deployments/"
  kind  = "post-covered"
  note  = "decided as a duplicate; nothing links the post or its announcement"
  as_of = "2026-11-19"
  state = "covered"
  work  = true
  expect {
    number = 39925
  }
}

case "post-sagemaker-inference-now-supports-multi-instance-gp" {
  post  = "https://aws.amazon.com/blogs/machine-learning/sagemaker-inference-now-supports-multi-instance-gpu-to-maximize-gpu-utilization-for-model-endpoints/"
  kind  = "post-gap"
  note  = "filed afterward as #51138, #51139"
  as_of = "2026-11-19T19:15:00Z"
  state = "open"
  work  = true
}

case "post-aws-weekly-roundup-agentcore-runtime-v2-amazon-dyn" {
  post  = "https://aws.amazon.com/blogs/aws/aws-weekly-roundup-agentcore-runtime-v2-amazon-dynamodb-bulk-put-items-and-more-november-9-2026/"
  kind  = "post-no-work"
  note  = "weekly roundup; its launches have their own posts and announcements"
  work  = false
}

case "post-amazon-athena-introduces-system-tables-for-query-e" {
  post  = "https://aws.amazon.com/blogs/big-data/amazon-athena-introduces-system-tables-for-query-execution-observability/"
  kind  = "post-covered"
  note  = "the issue links the post or its announcement"
  as_of = "2026-11-19"
  state = "covered"
  work  = true
  expect {
    number = 50887
  }
  expect {
    number = 50893
  }
}

case "post-assess-and-track-your-amazon-redshift-ra3-to-rg-mo" {
  post  = "https://aws.amazon.com/blogs/big-data/assess-and-track-your-amazon-redshift-ra3-to-rg-modernization-with-cudos/"
  kind  = "post-no-work"
  note  = "customer story or how-to guide"
  work  = false
}

case "post-introducing-the-workday-data-cloud-integration-wit" {
  post  = "https://aws.amazon.com/blogs/big-data/introducing-the-workday-data-cloud-integration-with-aws/"
  kind  = "post-no-work"
  note  = "customer story or how-to guide"
  work  = false
}

case "post-aws-nitro-sandboxes-hardware-enforced-workload-iso" {
  post  = "https://aws.amazon.com/blogs/containers/aws-nitro-sandboxes-hardware-enforced-workload-isolation-on-amazon-eks/"
  kind  = "post-covered"
  note  = "labeled in the coverage pass"
  as_of = "2026-11-19"
  state = "covered"
  work  = true
  expect {
    number = 50875
  }
  same  = [50971, 50973]
}

case "post-introducing-claude-fable-5-2-on-aws" {
  post  = "https://aws.amazon.com/blogs/machine-learning/introducing-claude-fable-5-2-on-aws/"
  kind  = "post-no-work"
  note  = "model availability"
  work  = false
}

case "post-nvidia-nemotron-3-5-super-vl-is-now-available-in-a" {
  post  = "https://aws.amazon.com/blogs/machine-learning/nvidia-nemotron-3-5-super-vl-is-now-available-in-amazon-sagemaker-jumpstart/"
  kind  = "post-no-work"
  note  = "model availability"
  work  = false
}

case "post-professional-work-deserves-professional-tools-meet" {
  post  = "https://aws.amazon.com/blogs/machine-learning/professional-work-deserves-professional-tools-meet-aws-quick/"
  kind  = "post-no-work"
  note  = "customer story or how-to guide"
  work  = false
}

case "post-simplify-cross-partition-access-with-aws-iam-outbo" {
  post  = "https://aws.amazon.com/blogs/security/simplify-cross-partition-access-with-aws-iam-outbound-identity-federation/"
  kind  = "post-covered"
  note  = "the issue links the post or its announcement"
  as_of = "2026-11-19"
  state = "covered"
  work  = true
  expect {
    number = 51078
  }
}

case "post-nitro-sandboxes-hardware-isolation-for-every-eks-p" {
  post  = "https://aws.amazon.com/blogs/compute/nitro-sandboxes-hardware-isolation-for-every-eks-pod-ecs-task-and-ec2-workload/"
  kind  = "post-covered"
  note  = "labeled in the coverage pass"
  as_of = "2026-11-19"
  state = "covered"
  work  = true
  expect {
    number = 50875
  }
  same  = [50971, 50973]
}

case "post-updated-default-retry-behavior-for-aws-sdks-and-to" {
  post  = "https://aws.amazon.com/blogs/developer/updated-default-retry-behavior-for-aws-sdks-and-tools/"
  kind  = "post-no-work"
  note  = "SDK or developer tooling"
  work  = false
  same  = [50905]
}

case "post-attendees-guide-to-aws-global-infrastructure-and-s" {
  post  = "https://aws.amazon.com/blogs/infrastructure-sustainability/attendees-guide-to-aws-global-infrastructure-and-sustainability-at-aws-reinvent-2026/"
  kind  = "post-no-work"
  note  = "event guide"
  work  = false
}

case "post-the-agent-evaluation-playbook-built-in-deepeval-an" {
  post  = "https://aws.amazon.com/blogs/machine-learning/the-agent-evaluation-playbook-built-in-deepeval-and-autoeval-on-agentcore/"
  kind  = "post-no-work"
  note  = "customer story or how-to guide"
  work  = false
}

case "post-aws-refreshes-processunity-global-risk-exchange-as" {
  post  = "https://aws.amazon.com/blogs/security/aws-refreshes-processunity-global-risk-exchange-assessment-to-enhance-customers-third-party-due-diligence/"
  kind  = "post-no-work"
  note  = "company, Region, or program news"
  work  = false
}

case "post-digital-sovereignty-at-reinvent-2026-sessions-you" {
  post  = "https://aws.amazon.com/blogs/security/digital-sovereignty-at-reinvent-2026-sessions-you-wont-want-to-miss/"
  kind  = "post-no-work"
  note  = "event guide"
  work  = false
}

case "post-introducing-aws-billing-conductor-and-aws-billing" {
  post  = "https://aws.amazon.com/blogs/aws-cloud-financial-management/introducing-aws-billing-conductor-and-aws-billing-transfer-views-as-source-views-for-custom-billing-views/"
  kind  = "post-no-work"
  note  = "aws_billing_view source_views takes the new view ARNs; no API change"
  work  = false
}

case "post-test-your-applications-against-aws-locally-with-aw" {
  post  = "https://aws.amazon.com/blogs/aws/test-your-applications-against-aws-locally-with-aws-emulator/"
  kind  = "post-no-work"
  note  = "SDK or developer tooling"
  work  = false
}

case "post-migrate-from-amazon-ec2-to-amazon-elasticache-serv" {
  post  = "https://aws.amazon.com/blogs/database/migrate-from-amazon-ec2-to-amazon-elasticache-serverless-with-zero-downtime/"
  kind  = "post-no-work"
  note  = "customer story or how-to guide"
  work  = false
}

case "post-migrating-epic-to-aws-returned-185-over-three-year" {
  post  = "https://aws.amazon.com/blogs/industries/migrating-epic-to-aws-returned-185-over-three-years-compared-to-on-premises-deployments/"
  kind  = "post-no-work"
  note  = "customer story or how-to guide"
  work  = false
}

case "post-icymi-what-landed-for-ai-builders-in-october-2026" {
  post  = "https://aws.amazon.com/blogs/machine-learning/icymi-what-landed-for-ai-builders-in-october-2026/"
  kind  = "post-no-work"
  note  = "no new resource or argument"
  work  = false
}

case "post-introducing-kimi-k3-1-pro-on-amazon-bedrock" {
  post  = "https://aws.amazon.com/blogs/machine-learning/introducing-kimi-k3-1-pro-on-amazon-bedrock/"
  kind  = "post-no-work"
  note  = "model availability"
  work  = false
}

case "post-introducing-the-web-search-api-on-amazon-bedrock-a" {
  post  = "https://aws.amazon.com/blogs/machine-learning/introducing-the-web-search-api-on-amazon-bedrock-agentcore/"
  kind  = "post-no-work"
  note  = "API call with no resources to create"
  work  = false
}

case "post-managed-cross-region-disaster-recovery-for-amazon" {
  post  = "https://aws.amazon.com/blogs/big-data/managed-cross-region-disaster-recovery-for-amazon-opensearch-service/"
  kind  = "post-covered"
  note  = "decided as a duplicate; nothing links the post or its announcement"
  as_of = "2026-11-19"
  state = "covered"
  work  = true
  expect {
    number = 50859
  }
}

case "post-from-vdi-to-workspaces-for-ai-agents-whats-next-fo" {
  post  = "https://aws.amazon.com/blogs/desktop-and-application-streaming/from-vdi-to-workspaces-for-ai-agents-whats-next-for-amazon-workspaces-at-reinvent-2026/"
  kind  = "post-no-work"
  note  = "event guide"
  work  = false
}

case "post-announcing-aws-sdk-for-net-4-1" {
  post  = "https://aws.amazon.com/blogs/developer/announcing-aws-sdk-for-net-4-1/"
  kind  = "post-no-work"
  note  = "SDK or developer tooling"
  work  = false
}

case "post-deploy-frontier-ai-models-on-aws-trainium-with-sgl" {
  post  = "https://aws.amazon.com/blogs/machine-learning/deploy-frontier-ai-models-on-aws-trainium-with-sglang-neuron/"
  kind  = "post-no-work"
  note  = "customer story or how-to guide"
  work  = false
}

case "post-run-agents-securely-and-efficiently-with-the-next" {
  post  = "https://aws.amazon.com/blogs/machine-learning/run-agents-securely-and-efficiently-with-the-next-generation-agentcore-runtime/"
  kind  = "post-gap"
  note  = "filed afterward as #51142"
  as_of = "2026-11-19T19:15:00Z"
  state = "open"
  work  = true
}

case "post-nfl-and-aws-introduce-next-gen-trends" {
  post  = "https://aws.amazon.com/blogs/media/nfl-and-aws-introduce-next-gen-trends/"
  kind  = "post-no-work"
  note  = "customer story or how-to guide"
  work  = false
}

case "post-resolving-cross-partition-dependencies-for-soverei" {
  post  = "https://aws.amazon.com/blogs/multicloud/resolving-cross-partition-dependencies-for-sovereign-failover-on-the-aws-european-sovereign-cloud/"
  kind  = "post-covered"
  note  = "the issue links the post or its announcement"
  as_of = "2026-11-19"
  state = "covered"
  work  = true
  expect {
    number = 51078
  }
}

case "post-bringing-ai-agents-closer-to-sap-whats-new-in-aws" {
  post  = "https://aws.amazon.com/blogs/awsforsap/bringing-ai-agents-closer-to-sap-whats-new-in-aws-for-sap-mcp-server/"
  kind  = "post-no-work"
  note  = "customer story or how-to guide"
  work  = false
}

case "post-multi-statement-transactions-for-apache-iceberg-ta" {
  post  = "https://aws.amazon.com/blogs/big-data/multi-statement-transactions-for-apache-iceberg-tables-in-amazon-redshift/"
  kind  = "post-no-work"
  note  = "no new resource or argument"
  work  = false
}

case "post-cloud-and-ai-accelerate-game-development-innovatio" {
  post  = "https://aws.amazon.com/blogs/gametech/cloud-and-ai-accelerate-game-development-innovation-at-aws-reinvent-2026/"
  kind  = "post-no-work"
  note  = "event guide"
  work  = false
}

case "post-your-guide-to-aws-cloud-resilience-sessions-at-rei" {
  post  = "https://aws.amazon.com/blogs/mt/your-guide-to-aws-cloud-resilience-sessions-at-reinvent-2026/"
  kind  = "post-no-work"
  note  = "event guide"
  work  = false
}

case "post-monitoring-global-traffic-with-aws-cloud-wan-flow" {
  post  = "https://aws.amazon.com/blogs/networking-and-content-delivery/monitoring-global-traffic-with-aws-cloud-wan-flow-logs/"
  kind  = "post-covered"
  note  = "labeled in the coverage pass"
  as_of = "2026-11-19"
  state = "covered"
  work  = true
  expect {
    number = 51011
  }
  same  = [51023]
}

case "post-built-in-europe-what-500-founders-tell-us-about-sc" {
  post  = "https://aws.amazon.com/blogs/physical-ai/built-in-europe-what-500-founders-tell-us-about-scaling-physical-ai/"
  kind  = "post-no-work"
  note  = "customer story or how-to guide"
  work  = false
}

case "post-amazon-threat-intelligence-observes-threat-actors" {
  post  = "https://aws.amazon.com/blogs/security/amazon-threat-intelligence-observes-threat-actors-running-cybercrime-campaigns-with-artex-ai-agents/"
  kind  = "post-no-work"
  note  = "company, Region, or program news"
  work  = false
}

case "post-buckets-guide-to-whats-in-store-with-aws-storage-a" {
  post  = "https://aws.amazon.com/blogs/storage/buckets-guide-to-whats-in-store-with-aws-storage-at-reinvent-2026/"
  kind  = "post-no-work"
  note  = "event guide"
  work  = false
}

case "post-ja-canada-launches-generative-ai-with-qalius-and-a" {
  post  = "https://aws.amazon.com/blogs/apn/ja-canada-launches-generative-ai-with-qalius-and-amazon-bedrock/"
  kind  = "post-no-work"
  note  = "customer story or how-to guide"
  work  = false
}

case "post-announcing-updates-to-the-aws-well-architected-fra" {
  post  = "https://aws.amazon.com/blogs/architecture/announcing-updates-to-the-aws-well-architected-framework-3/"
  kind  = "post-covered"
  note  = "labeled in the coverage pass"
  as_of = "2026-11-19"
  state = "covered"
  work  = true
  expect {
    number = 51057
  }
}

case "post-aws-weekly-roundup-aws-nitro-sandboxes-aws-emulato" {
  post  = "https://aws.amazon.com/blogs/aws/aws-weekly-roundup-aws-nitro-sandboxes-aws-emulator-and-more-november-16-2026/"
  kind  = "post-no-work"
  note  = "weekly roundup; its launches have their own posts and announcements"
  work  = false
}

case "post-aws-marketplace-in-singapore-sgd-support-local-inv" {
  post  = "https://aws.amazon.com/blogs/awsmarketplace/aws-marketplace-in-singapore-sgd-support-local-invoicing-and-gst-compliance/"
  kind  = "post-no-work"
  note  = "company, Region, or program news"
  work  = false
}

case "post-deliver-petabytes-of-apache-kafka-data-to-amazon-s" {
  post  = "https://aws.amazon.com/blogs/big-data/deliver-petabytes-of-apache-kafka-data-to-amazon-s3-at-up-to-60-lower-cost-with-amazon-msk-express-brokers/"
  kind  = "post-covered"
  note  = "decided as a duplicate; nothing links the post or its announcement"
  as_of = "2026-11-19"
  state = "covered"
  work  = true
  expect {
    number = 49266
  }
}

case "post-introducing-enhanced-snapshots-for-amazon-managed" {
  post  = "https://aws.amazon.com/blogs/big-data/introducing-enhanced-snapshots-for-amazon-managed-service-for-apache-flink/"
  kind  = "post-covered"
  note  = "labeled in the coverage pass"
  as_of = "2026-11-19"
  state = "covered"
  work  = true
  expect {
    number = 51003
  }
  same  = [51073]
}

case "post-migrate-apache-solr-to-amazon-opensearch-service-w" {
  post  = "https://aws.amazon.com/blogs/big-data/migrate-apache-solr-to-amazon-opensearch-service-with-the-ai-driven-migration-assistant/"
  kind  = "post-no-work"
  note  = "customer story or how-to guide"
  work  = false
}

case "post-exploring-new-features-of-apache-tinkerpop-3-8-x-i" {
  post  = "https://aws.amazon.com/blogs/database/exploring-new-features-of-apache-tinkerpop-3-8-x-in-amazon-neptune/"
  kind  = "post-no-work"
  note  = "engine or instance type version"
  work  = false
}

case "post-workspaces-for-ai-agents-now-available-on-workspac" {
  post  = "https://aws.amazon.com/blogs/desktop-and-application-streaming/workspaces-for-ai-agents-now-available-on-workspaces-applications-image-builders/"
  kind  = "post-gap"
  note  = "filed afterward as #51131, #51132"
  as_of = "2026-11-19T19:15:00Z"
  state = "open"
  work  = true
}

case "post-how-to-make-four-specialist-ai-agents-act-as-one-t" {
  post  = "https://aws.amazon.com/blogs/industries/how-to-make-four-specialist-ai-agents-act-as-one-trusted-financial-advisor/"
  kind  = "post-no-work"
  note  = "customer story or how-to guide"
  work  = false
}

case "post-customize-answers-per-tenant-in-amazon-bedrock-man" {
  post  = "https://aws.amazon.com/blogs/machine-learning/customize-answers-per-tenant-in-amazon-bedrock-managed-knowledge-base/"
  kind  = "post-covered"
  note  = "the issue links the post or its announcement"
  as_of = "2026-11-19"
  state = "covered"
  work  = true
  expect {
    number = 50992
  }
  expect {
    number = 50993
  }
}

case "post-use-prompt-caching-with-open-weight-models-on-amaz" {
  post  = "https://aws.amazon.com/blogs/machine-learning/use-prompt-caching-with-open-weight-models-on-amazon-bedrock/"
  kind  = "post-no-work"
  note  = "no new resource or argument"
  work  = false
}

case "post-introducing-11-nat-on-aws-transit-gateway-and-aws" {
  post  = "https://aws.amazon.com/blogs/networking-and-content-delivery/introducing-11-nat-on-aws-transit-gateway-and-aws-cloud-wan/"
  kind  = "post-covered"
  note  = "labeled in the coverage pass"
  as_of = "2026-11-19"
  state = "covered"
  work  = true
  expect {
    number = 50613
  }
  same  = [50669]
}

case "post-introducing-aws-backup-logically-air-gapped-vault" {
  post  = "https://aws.amazon.com/blogs/storage/introducing-aws-backup-logically-air-gapped-vault-support-for-amazon-rds/"
  kind  = "post-no-work"
  note  = "existing logically air-gapped vault and backup plan resources; no API change"
  work  = false
}

case "post-four-continents-four-new-aws-heroes-closing-out-20" {
  post  = "https://aws.amazon.com/blogs/aws/four-continents-four-new-aws-heroes-closing-out-2026/"
  kind  = "post-no-work"
  note  = "company, Region, or program news"
  work  = false
}

case "post-amazon-msk-express-brokers-available-on-aws-gravit" {
  post  = "https://aws.amazon.com/blogs/big-data/amazon-msk-express-brokers-available-on-aws-graviton4-based-m8g-instances/"
  kind  = "post-no-work"
  note  = "engine or instance type version"
  work  = false
}

case "post-reduce-the-complexity-and-cost-of-real-time-proces" {
  post  = "https://aws.amazon.com/blogs/big-data/reduce-the-complexity-and-cost-of-real-time-processing-with-queues-in-amazon-kinesis-data-streams/"
  kind  = "post-covered"
  note  = "labeled in the coverage pass"
  as_of = "2026-11-19"
  state = "covered"
  work  = true
  expect {
    number = 51120
  }
}

case "post-securing-rest-apis-with-jwt-authorizers-in-amazon" {
  post  = "https://aws.amazon.com/blogs/compute/securing-rest-apis-with-jwt-authorizers-in-amazon-api-gateway/"
  kind  = "post-covered"
  note  = "labeled in the coverage pass"
  as_of = "2026-11-19"
  state = "covered"
  work  = true
  expect {
    number = 51086
  }
}

case "post-accelerate-your-dynamodb-workloads-with-dynamodb-c" {
  post  = "https://aws.amazon.com/blogs/database/accelerate-your-dynamodb-workloads-with-dynamodb-caching-with-amazon-elasticache-serverless-for-valkey/"
  kind  = "post-covered"
  note  = "labeled in the coverage pass"
  as_of = "2026-11-19"
  state = "covered"
  work  = true
  expect {
    number = 51117
  }
}

case "post-use-query-statistics-to-find-slow-and-expensive-qu" {
  post  = "https://aws.amazon.com/blogs/database/use-query-statistics-to-find-slow-and-expensive-queries-in-aurora-dsql/"
  kind  = "post-covered"
  note  = "labeled in the coverage pass"
  as_of = "2026-11-19"
  state = "covered"
  work  = true
  expect {
    number = 51095
  }
}

case "post-introducing-smithy-idl-2-1" {
  post  = "https://aws.amazon.com/blogs/developer/introducing-smithy-idl-2-1/"
  kind  = "post-no-work"
  note  = "SDK or developer tooling"
  work  = false
}

case "post-one-call-no-waiting-amazon-connect-health-expands" {
  post  = "https://aws.amazon.com/blogs/industries/one-call-no-waiting-amazon-connect-health-expands-agentic-ai-for-patient-access/"
  kind  = "post-no-work"
  note  = "Connect Health runtime features; service not in the provider"
  work  = false
}

case "post-amazon-sagemaker-ai-real-time-endpoints-now-suppor" {
  post  = "https://aws.amazon.com/blogs/machine-learning/amazon-sagemaker-ai-real-time-endpoints-now-support-kv-cache-aware-routing/"
  kind  = "post-covered"
  note  = "decided as a duplicate; nothing links the post or its announcement"
  as_of = "2026-11-19"
  state = "covered"
  work  = true
  expect {
    number = 51090
  }
}

case "post-announcing-ai-data-preparation-capabilities-in-ama" {
  post  = "https://aws.amazon.com/blogs/machine-learning/announcing-ai-data-preparation-capabilities-in-amazon-sagemaker-build-high-quality-datasets-for-model-development-workflows/"
  kind  = "post-no-work"
  note  = "data preparation jobs run from the SDK; no resource"
  work  = false
}

case "post-managed-nvidia-dynamo-on-amazon-sagemaker-hyperpod" {
  post  = "https://aws.amazon.com/blogs/machine-learning/managed-nvidia-dynamo-on-amazon-sagemaker-hyperpod-inference/"
  kind  = "post-no-work"
  note  = "installed with the HyperPod Inference EKS add-on (aws_eks_addon) and configured as Kubernetes custom resources"
  work  = false
}

case "post-introducing-simplified-onboarding-for-aws-waf-bot" {
  post  = "https://aws.amazon.com/blogs/security/introducing-simplified-onboarding-for-aws-waf-bot-control-at-the-targeted-inspection-level/"
  kind  = "post-no-work"
  note  = "managed rule group version and labels; set with the existing managed_rule_group_statement"
  work  = false
}

case "post-using-the-aws-rmit-workbook-and-iso-compliance-gui" {
  post  = "https://aws.amazon.com/blogs/security/using-the-aws-rmit-workbook-and-iso-compliance-guides-to-support-your-bnm-submissions/"
  kind  = "post-no-work"
  note  = "company, Region, or program news"
  work  = false
}

case "post-block-dns-threats-with-infoblox-rules-in-amazon-ro" {
  post  = "https://aws.amazon.com/blogs/architecture/block-dns-threats-with-infoblox-rules-in-amazon-route-53-resolver-dns-firewall/"
  kind  = "post-gap"
  note  = "deferred, preview; no SDK change yet"
  as_of = "2026-11-19T19:15:00Z"
  state = "open"
  work  = true
}

case "post-aws-lambda-now-supports-gpu-instances-for-event-fu" {
  post  = "https://aws.amazon.com/blogs/aws/aws-lambda-now-supports-gpu-instances-for-event-functions-on-lambda-managed-instances/"
  kind  = "post-covered"
  note  = "labeled in the coverage pass"
  as_of = "2026-11-19"
  state = "covered"
  work  = true
  expect {
    number = 51046
  }
  same  = [51048]
}

case "post-introducing-amazon-opensearch-memory-persistent-me" {
  post  = "https://aws.amazon.com/blogs/big-data/introducing-amazon-opensearch-memory-persistent-memory-for-ai-agents/"
  kind  = "post-covered"
  note  = "labeled in the coverage pass"
  as_of = "2026-11-19"
  state = "covered"
  work  = true
  expect {
    number = 51056
  }
}

case "post-introducing-the-apache-spark-optimization-agent-fo" {
  post  = "https://aws.amazon.com/blogs/big-data/introducing-the-apache-spark-optimization-agent-for-amazon-emr-serverless-and-aws-glue-etl/"
  kind  = "post-no-work"
  note  = "no new resource or argument"
  work  = false
}

case "post-long-term-amazon-redshift-observability-with-datad" {
  post  = "https://aws.amazon.com/blogs/big-data/long-term-amazon-redshift-observability-with-datadog-and-amazon-s3-tables/"
  kind  = "post-no-work"
  note  = "customer story or how-to guide"
  work  = false
}

case "post-control-your-aws-lambda-runtimes-with-customer-man" {
  post  = "https://aws.amazon.com/blogs/compute/control-your-aws-lambda-runtimes-with-customer-managed-runtimes/"
  kind  = "post-covered"
  note  = "labeled in the coverage pass"
  as_of = "2026-11-19"
  state = "covered"
  work  = true
  expect {
    number = 51040
  }
  expect {
    number = 51043
  }
  same  = [51041, 51042]
}

case "post-agentic-ccaas-the-new-category-for-contact-center" {
  post  = "https://aws.amazon.com/blogs/contact-center/agentic-ccaas-the-new-category-for-contact-center-platforms-and-why-it-matters/"
  kind  = "post-no-work"
  note  = "company, Region, or program news"
  work  = false
}

case "post-amazon-rds-for-mysql-lts-version-9-7-is-now-genera" {
  post  = "https://aws.amazon.com/blogs/database/amazon-rds-for-mysql-lts-version-9-7-is-now-generally-available/"
  kind  = "post-no-work"
  note  = "engine or instance type version"
  work  = false
}

case "post-announcing-distributed-kv-caching-for-self-hosted" {
  post  = "https://aws.amazon.com/blogs/database/announcing-distributed-kv-caching-for-self-hosted-llms-with-amazon-elasticache/"
  kind  = "post-no-work"
  note  = "client-side library; no API change"
  work  = false
}

case "post-announcing-developer-preview-aws-sdk-for-python-v4" {
  post  = "https://aws.amazon.com/blogs/developer/announcing-developer-preview-aws-sdk-for-python-v4/"
  kind  = "post-no-work"
  note  = "SDK or developer tooling"
  work  = false
}

case "post-announcing-the-developer-preview-of-transfer-manag" {
  post  = "https://aws.amazon.com/blogs/developer/announcing-the-developer-preview-of-transfer-manager-for-amazon-s3-in-the-aws-sdk-for-javascript-v3/"
  kind  = "post-no-work"
  note  = "SDK or developer tooling"
  work  = false
}

case "post-high-throughput-transfer-manager-for-amazon-s3-in" {
  post  = "https://aws.amazon.com/blogs/developer/high-throughput-transfer-manager-for-amazon-s3-in-rust-is-now-generally-available/"
  kind  = "post-no-work"
  note  = "SDK or developer tooling"
  work  = false
}

case "post-physical-ai-production-designer-cell-programming-f" {
  post  = "https://aws.amazon.com/blogs/industries/physical-ai-production-designer-cell-programming-from-weeks-to-hours/"
  kind  = "post-no-work"
  note  = "customer story or how-to guide"
  work  = false
}

case "post-your-guide-to-retail-and-consumer-goods-at-reinven" {
  post  = "https://aws.amazon.com/blogs/industries/your-guide-to-retail-and-consumer-goods-at-reinvent-2026/"
  kind  = "post-no-work"
  note  = "event guide"
  work  = false
}

case "post-from-reporting-to-reasoning-getting-enterprise-dat" {
  post  = "https://aws.amazon.com/blogs/machine-learning/from-reporting-to-reasoning-getting-enterprise-data-ready-for-ai-agents/"
  kind  = "post-no-work"
  note  = "customer story or how-to guide"
  work  = false
}

case "post-how-salesforce-is-advancing-global-scale-model-ser" {
  post  = "https://aws.amazon.com/blogs/machine-learning/how-salesforce-is-advancing-global-scale-model-serving-with-cross-region-inference-routing-in-amazon-sagemaker-ai/"
  kind  = "post-covered"
  note  = "the issue links the post or its announcement"
  as_of = "2026-11-19"
  state = "covered"
  work  = true
  expect {
    number = 51090
  }
  expect {
    number = 51091
  }
  expect {
    number = 51092
  }
}

case "post-introducing-the-amazon-sagemaker-jumpstart-model-l" {
  post  = "https://aws.amazon.com/blogs/machine-learning/introducing-the-amazon-sagemaker-jumpstart-model-leaderboard/"
  kind  = "post-no-work"
  note  = "model availability"
  work  = false
}

case "post-introducing-workload-recovery-and-expanded-infrast" {
  post  = "https://aws.amazon.com/blogs/machine-learning/introducing-workload-recovery-and-expanded-infrastructure-checks-on-amazon-sagemaker-hyperpod/"
  kind  = "post-gap"
  note  = "filed afterward as #51143"
  as_of = "2026-11-19T19:15:00Z"
  state = "open"
  work  = true
}

case "post-governing-your-ai-workloads-on-aws-at-scale-visibi" {
  post  = "https://aws.amazon.com/blogs/mt/governing-your-ai-workloads-on-aws-at-scale-visibility-controls-and-monitoring/"
  kind  = "post-covered"
  note  = "the issue links the post or its announcement"
  as_of = "2026-11-19"
  state = "covered"
  work  = true
  expect {
    number = 51116
  }
}

case "post-hybrid-dns-with-amazon-route-53-global-resolver-fo" {
  post  = "https://aws.amazon.com/blogs/networking-and-content-delivery/hybrid-dns-with-amazon-route-53-global-resolver-for-private-connections-on-aws-cloud-wan/"
  kind  = "post-covered"
  note  = "decided as a duplicate; nothing links the post or its announcement"
  as_of = "2026-11-19"
  state = "covered"
  work  = true
  expect {
    number = 45334
  }
}

case "post-introducing-network-calls-in-amazon-cloudfront-fun" {
  post  = "https://aws.amazon.com/blogs/networking-and-content-delivery/introducing-network-calls-in-amazon-cloudfront-functions/"
  kind  = "post-no-work"
  note  = "used in function code; no API change"
  work  = false
}

case "post-new-site-to-site-vpn-connections-will-default-to-r" {
  post  = "https://aws.amazon.com/blogs/networking-and-content-delivery/new-site-to-site-vpn-connections-will-default-to-recommended-cryptography/"
  kind  = "post-no-work"
  note  = "no new resource or argument"
  work  = false
}

case "post-aws-named-a-leader-in-idc-marketscape-for-public-c" {
  post  = "https://aws.amazon.com/blogs/storage/aws-named-a-leader-in-idc-marketscape-for-public-cloud-object-storage/"
  kind  = "post-no-work"
  note  = "company, Region, or program news"
  work  = false
}

case "post-4-year-anniversary-news-about-aws-europe-zurich-re" {
  post  = "https://aws.amazon.com/blogs/alps/4-year-anniversary-news-about-aws-europe-zurich-region/"
  kind  = "post-no-work"
  note  = "company, Region, or program news"
  work  = false
}

case "post-introducing-automatic-version-upgrades-for-aws-glu" {
  post  = "https://aws.amazon.com/blogs/big-data/introducing-automatic-version-upgrades-for-aws-glue-jobs/"
  kind  = "post-gap"
  note  = "filed afterward as #51140"
  as_of = "2026-11-19T19:15:00Z"
  state = "open"
  work  = true
}

case "post-monitor-the-efficiency-and-availability-of-acceler" {
  post  = "https://aws.amazon.com/blogs/compute/monitor-the-efficiency-and-availability-of-accelerated-computing-instances-with-amazon-ec2-detailed-monitoring-and-performance-status-checks/"
  kind  = "post-gap"
  note  = "filed afterward as #51135, #51136, #51137"
  as_of = "2026-11-19T19:15:00Z"
  state = "open"
  work  = true
}

case "post-amazon-elasticache-serverless-turns-three" {
  post  = "https://aws.amazon.com/blogs/database/amazon-elasticache-serverless-turns-three/"
  kind  = "post-no-work"
  note  = "no new resource or argument"
  work  = false
}

case "post-full-fine-tuning-fft-on-sagemaker-ai-serverless-mo" {
  post  = "https://aws.amazon.com/blogs/machine-learning/full-fine-tuning-fft-on-sagemaker-ai-serverless-model-customization/"
  kind  = "post-no-work"
  note  = "no new resource or argument"
  work  = false
}

case "post-introducing-enhanced-observability-in-amazon-sagem" {
  post  = "https://aws.amazon.com/blogs/machine-learning/introducing-enhanced-observability-in-amazon-sagemaker-hyperpod-to-diagnose-gpu-training-issues/"
  kind  = "post-gap"
  note  = "filed afterward as #51143"
  as_of = "2026-11-19T19:15:00Z"
  state = "open"
  work  = true
}

case "post-polyai-dialog-rsn-1-is-now-on-amazon-sagemaker-jum" {
  post  = "https://aws.amazon.com/blogs/machine-learning/polyai-dialog-rsn-1-is-now-on-amazon-sagemaker-jumpstart/"
  kind  = "post-no-work"
  note  = "model availability"
  work  = false
}

case "post-introducing-resource-context-for-firewall-policies" {
  post  = "https://aws.amazon.com/blogs/networking-and-content-delivery/introducing-resource-context-for-firewall-policies-with-aws-transit-gateway/"
  kind  = "post-covered"
  note  = "decided as a duplicate; nothing links the post or its announcement"
  as_of = "2026-11-19"
  state = "covered"
  work  = true
  expect {
    number = 51064
  }
  expect {
    number = 51066
  }
}
