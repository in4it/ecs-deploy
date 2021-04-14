#
# ecs-deploy
#

#
# accountid
#
data "aws_caller_identity" "current" {
}

#
# ECS service
#

resource "aws_ecs_service" "ecs-deploy" {
  name                               = "ecs-deploy"
  cluster                            = aws_ecs_cluster.cluster.id
  task_definition                    = var.ecs_deploy_enable_appmesh ? aws_ecs_task_definition.ecs-deploy-appmesh[0].arn : aws_ecs_task_definition.ecs-deploy[0].arn
  iam_role                           = var.ecs_deploy_awsvpc ? "" : aws_iam_role.cluster-service-role.arn
  desired_count                      = 1
  deployment_minimum_healthy_percent = 100
  deployment_maximum_percent         = 200

  dynamic "network_configuration" {
    for_each = var.ecs_deploy_awsvpc ? ["enabled"] : []
    content {
      subnets          = var.vpc_private_subnets
      security_groups  = [aws_security_group.ecs-deploy-awsvpc.id]
      assign_public_ip = false
    }
  }

  dynamic "service_registries" {
    for_each = var.ecs_deploy_service_discovery_id == "" ? [] : ["enabled"]
    content {
      registry_arn = aws_service_discovery_service.ecs-deploy[0].arn
    }
  }

  load_balancer {
    target_group_arn = aws_alb_target_group.ecs-deploy.id
    container_name   = "ecs-deploy"
    container_port   = 8080
  }
}

locals {
  template_vars = {
    AWS_REGION            = var.aws_region
    ENVIRONMENT           = var.aws_env
    PARAMSTORE_ENABLED    = var.paramstore_enabled
    CLUSTER_NAME          = var.cluster_name
    ECS_DEPLOY_IMAGE      = var.ecs_deploy_image
    ECS_DEPLOY_VERSION    = var.ecs_deploy_version
    DEBUG                 = var.ecs_deploy_debug
    APPMESH_NAME          = var.ecs_deploy_appmesh_name
    APPMESH_ENVOY_RELEASE = var.ecs_deploy_appmesh_release
    ECS_WHITELIST         = var.ecs_whitelist

    ECS_DEPLOY_CPU                = var.ecs_deploy_cpu
    ECS_DEPLOY_MEMORY_RESERVATION = var.ecs_deploy_memory_reservation
  }
}

resource "aws_ecs_task_definition" "ecs-deploy" {
  count                 = var.ecs_deploy_enable_appmesh ? 0 : 1
  family                = "ecs-deploy"
  container_definitions = var.ecs_deploy_enable_appmesh ? templatefile("${path.module}/templates/ecs-deploy-appmesh.json", local.template_vars) : templatefile("${path.module}/templates/ecs-deploy.json", local.template_vars)
  task_role_arn         = aws_iam_role.ecs-deploy.arn
  network_mode          = var.ecs_deploy_awsvpc ? "awsvpc" : "bridge"
  execution_role_arn    = var.ecs_deploy_awsvpc ? aws_iam_role.ecs-task-execution-role.arn : ""
}

resource "aws_ecs_task_definition" "ecs-deploy-appmesh" {
  count                 = var.ecs_deploy_enable_appmesh ? 1 : 0
  family                = "ecs-deploy"
  container_definitions = var.ecs_deploy_enable_appmesh ? templatefile("${path.module}/templates/ecs-deploy-appmesh.json", local.template_vars) : templatefile("${path.module}/templates/ecs-deploy.json", local.template_vars)
  task_role_arn         = aws_iam_role.ecs-deploy.arn
  network_mode          = var.ecs_deploy_awsvpc ? "awsvpc" : "bridge"
  execution_role_arn    = var.ecs_deploy_awsvpc ? aws_iam_role.ecs-task-execution-role.arn : ""

  proxy_configuration {
    type           = "APPMESH"
    container_name = "envoy"
    properties = {
      AppPorts         = "10000"
      EgressIgnoredIPs = "169.254.170.2,169.254.169.254"
      IgnoredUID       = "1337"
      ProxyEgressPort  = 15001
      ProxyIngressPort = 15000
    }
  }
}

#
# IAM role & policy
#
resource "aws_iam_role" "ecs-deploy" {
  name = "ecs-deploy"

  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": "sts:AssumeRole",
      "Principal": {
        "Service": "ecs-tasks.amazonaws.com"
      },
      "Effect": "Allow",
      "Sid": ""
    }
  ]
}
EOF

}

resource "aws_iam_role_policy" "ecs-deploy-policy" {
  name = "ecs-deploy-policy"
  role = aws_iam_role.ecs-deploy.id

  policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "ecs:DeregisterContainerInstance",
        "ecs:DiscoverPollEndpoint",
        "ecs:Poll",
        "ecs:RegisterContainerInstance",
        "ecs:StartTelemetrySession",
        "ecs:Submit*",
        "ecs:StartTask",
        "ecs:Describe*",
        "ecs:List*",
        "ecs:UpdateService",
        "ecs:CreateService",
        "ecs:RegisterTaskDefinition",
        "ecs:UpdateContainerInstancesState",
        "ecr:GetAuthorizationToken",
        "ecr:BatchCheckLayerAvailability",
        "ecr:GetDownloadUrlForLayer",
        "ecr:GetRepositoryPolicy",
        "ecr:DescribeRepositories",
        "ecr:ListImages",
        "ecr:DescribeImages",
        "ecr:BatchGetImage",
        "ecr:InitiateLayerUpload",
        "ecr:UploadLayerPart",
        "ecr:CompleteLayerUpload",
        "ecr:PutImage",
        "ecr:CreateRepository",
        "ecr:PutLifecyclePolicy",
        "elasticloadbalancing:Describe*",
        "elasticloadbalancing:CreateRule",
        "elasticloadbalancing:DeleteRule",
        "elasticloadbalancing:CreateTargetGroup",
        "elasticloadbalancing:DeleteTargetGroup",
        "elasticloadbalancing:ModifyTargetGroupAttributes",
        "acm:DescribeCertificate",
        "autoscaling:DescribeAutoScalingGroups",
        "autoscaling:DescribeLifecycleHooks",
        "autoscaling:DescribeAutoScalingNotificationTypes",
        "autoscaling:UpdateAutoScalingGroup",
        "autoscaling:CompleteLifecycleAction",
        "logs:GetLogEvents",
        "ec2:DescribeTags",
        "ec2:DescribeSecurityGroups",
        "ec2:DescribeSubnets",
        "cloudwatch:PutMetricAlarm",
        "cloudwatch:DescribeAlarms",
        "cloudwatch:DeleteAlarms",
        "application-autoscaling:PutScalingPolicy",
        "application-autoscaling:RegisterScalableTarget",
        "application-autoscaling:DeregisterScalableTarget",
        "application-autoscaling:DescribeScalableTargets",
        "application-autoscaling:DescribeScalingPolicies",
        "application-autoscaling:DeleteScalingPolicy",
        "servicediscovery:ListNamespaces",
        "servicediscovery:ListServices",
        "servicediscovery:CreateService",
        "ssm:GetParametersByPath",
        "cognito-idp:DescribeUserPool",
        "cognito-idp:DescribeUserPoolClient",
        "cognito-idp:ListUserPoolClients",
        "cognito-idp:ListUserPools",
        "appmesh:CreateVirtualNode",
        "appmesh:CreateVirtualService",
        "appmesh:CreateVirtualRouter",
        "appmesh:CreateRoute",
        "appmesh:UpdateVirtualNode",
        "appmesh:UpdateVirtualService",
        "appmesh:UpdateVirtualRouter",
        "appmesh:UpdateRoute",
        "appmesh:ListVirtualNodes",
        "appmesh:ListVirtualServices",
        "appmesh:ListVirtualNodes",
        "appmesh:ListVirtualServices",
        "appmesh:ListVirtualRouters",
        "appmesh:ListRoutes"
      ],
      "Resource": "*"
    },
    {
      "Effect": "Allow",
      "Action": [
          "iam:CreateRole",
          "iam:AttachRolePolicy",
          "iam:PutRolePolicy",
          "iam:GetRole",
          "iam:PassRole"
      ],
      "Resource": "arn:aws:iam::*:role/ecs-*"
    },
    {
      "Action": [
        "ssm:GetParameterHistory",
        "ssm:GetParameter",
        "ssm:GetParameters",
        "ssm:GetParametersByPath"
      ],
      "Resource": [
        "arn:aws:ssm:${var.aws_region}:${data.aws_caller_identity.current.account_id}:parameter/${var.cluster_name}-${var.aws_env}/ecs-deploy/*"
      ],
      "Effect": "Allow"
    },
    {
      "Action": [
        "kms:Decrypt"
      ],
      "Resource": [
        "${data.aws_kms_key.ssm.arn}"
      ],
      "Effect": "Allow"
    },
    {
      "Effect": "Allow",
      "Action": [
          "dynamodb:*"
      ],
      "Resource": [
        "arn:aws:dynamodb:${var.aws_region}:${data.aws_caller_identity.current.account_id}:table/ecs-deploy",
        "arn:aws:dynamodb:${var.aws_region}:${data.aws_caller_identity.current.account_id}:table/ecs-deploy/*"
      ]
    }
  ]
}
EOF

}

#
# dynamodb
#
resource "aws_dynamodb_table" "ecs-deploy" {
  name           = "ecs-deploy"
  read_capacity  = var.dynamodb_read_capacity
  write_capacity = var.dynamodb_write_capacity
  hash_key       = "ServiceName"
  range_key      = "Time"
  server_side_encryption {
    enabled = var.enable_dynamodb_encryption
  }

  attribute {
    name = "ServiceName"
    type = "S"
  }

  attribute {
    name = "Time"
    type = "S"
  }

  attribute {
    name = "Day"
    type = "S"
  }

  attribute {
    name = "Month"
    type = "S"
  }

  ttl {
    attribute_name = "ExpirationTimeTTL"
    enabled        = true
  }

  global_secondary_index {
    name            = "DayIndex"
    hash_key        = "Day"
    range_key       = "Time"
    write_capacity  = 2
    read_capacity   = 2
    projection_type = "ALL"
  }

  global_secondary_index {
    name            = "MonthIndex"
    hash_key        = "Month"
    range_key       = "Time"
    write_capacity  = 2
    read_capacity   = 2
    projection_type = "ALL"
  }
}

# cloudwatch log group
resource "aws_cloudwatch_log_group" "ecs-deploy" {
  name = "ecs-deploy"
}

#
# webhook and autoscaling hook
#
# sns topic for ecs events
resource "aws_sns_topic" "ecs-deploy" {
  count = var.ecs_capacity_provider_enabled ? 0 : 1
  name = "ecs-deploy-events"

  policy = <<EOF
{
  "Version": "2008-10-17",
  "Id": "__default_policy_ID",
  "Statement": [
    {
      "Sid": "__default_statement_ID",
      "Effect": "Allow",
      "Principal": {
        "AWS": "*"
      },
      "Action": [
        "SNS:GetTopicAttributes",
        "SNS:SetTopicAttributes",
        "SNS:AddPermission",
        "SNS:RemovePermission",
        "SNS:DeleteTopic",
        "SNS:Subscribe",
        "SNS:ListSubscriptionsByTopic",
        "SNS:Publish",
        "SNS:Receive"
      ],
      "Resource": "arn:aws:sns:${var.aws_region}:${data.aws_caller_identity.current.account_id}:ecs-deploy-events",
      "Condition": {
        "StringEquals": {
          "AWS:SourceOwner": "${data.aws_caller_identity.current.account_id}"
        }
      }
    },
    {
      "Sid": "TrustCWEToPublishEventsToMyTopic",
      "Effect": "Allow",
      "Principal": {
        "Service": "events.amazonaws.com"
      },
      "Action": "sns:Publish",
      "Resource": "arn:aws:sns:${var.aws_region}:${data.aws_caller_identity.current.account_id}:ecs-deploy-events"
    }
  ]
}
EOF

}

# post sns to ecs-deploy(https)
resource "aws_sns_topic_subscription" "ecs-deploy" {
  count = var.ecs_capacity_provider_enabled ? 0 : 1
  topic_arn              = aws_sns_topic.ecs-deploy[0].arn
  protocol               = "https"
  endpoint               = "https://${var.sns_endpoint == "" ? var.cluster_domain : var.sns_endpoint}/ecs-deploy/webhook"
  endpoint_auto_confirms = true
}

# Watch for ecs events in the logs
resource "aws_cloudwatch_event_rule" "ecs-deploy" {
  count = var.ecs_capacity_provider_enabled ? 0 : 1
  name        = "ecs-event"
  description = "Capture ecs events"

  event_pattern = <<PATTERN
{
  "source": [
    "aws.ecs"
  ],
  "detail-type": [
    "ECS Container Instance State Change",
    "ECS Task State Change"
  ],
  "detail": {
    "clusterArn": [
      "${aws_ecs_cluster.cluster.id}"
    ]
  }
}
PATTERN

}

# Send ecs-events to sns
resource "aws_cloudwatch_event_target" "ecs-deploy" {
  count = var.ecs_capacity_provider_enabled ? 0 : 1
  rule      = aws_cloudwatch_event_rule.ecs-deploy[0].name
  target_id = "SendEcsEventToSNS"
  arn       = aws_sns_topic.ecs-deploy[0].arn
}

# cloudwatch event for autoscaling
resource "aws_cloudwatch_event_rule" "ecs-deploy-autoscaling" {
  count = var.ecs_capacity_provider_enabled ? 0 : 1
  name        = "ecs-deploy-autoscaling"
  description = "Capture autoscaling events"

  event_pattern = <<PATTERN
{
  "source": [
    "aws.autoscaling"
  ],
  "detail-type": [
    "EC2 Instance-terminate Lifecycle Action"
  ],
  "detail": {
    "LifecycleHookName": [
      "${aws_autoscaling_lifecycle_hook.cluster[0].name}"
    ]
  }
}
PATTERN

}

# Send ecs-events to sns
resource "aws_cloudwatch_event_target" "ecs-deploy-autoscaling" {
  count = var.ecs_capacity_provider_enabled ? 0 : 1
  rule      = aws_cloudwatch_event_rule.ecs-deploy-autoscaling[0].name
  target_id = "SendAutoscalingEventToSNS"
  arn       = aws_sns_topic.ecs-deploy[0].arn
}

