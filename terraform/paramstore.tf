# url prefix
resource "aws_ssm_parameter" "ecs-deploy-url-prefix" {
  name  = "/${var.cluster_name}-${var.aws_env}/ecs-deploy/URL_PREFIX"
  type  = "String"
  value = var.url_prefix
}

# env
resource "aws_ssm_parameter" "ecs-deploy-aws-account-env" {
  name  = "/${var.cluster_name}-${var.aws_env}/ecs-deploy/AWS_ACCOUNT_ENV"
  type  = "String"
  value = var.aws_env
}

# paramstore config
resource "aws_ssm_parameter" "ecs-deploy-paramstore-prefix" {
  name  = "/${var.cluster_name}-${var.aws_env}/ecs-deploy/PARAMSTORE_PREFIX"
  type  = "String"
  value = var.cluster_name
}

resource "aws_ssm_parameter" "ecs-deploy-kms-id" {
  name  = "/${var.cluster_name}-${var.aws_env}/ecs-deploy/PARAMSTORE_KMS_ARN"
  type  = "String"
  value = data.aws_kms_key.ssm.arn
}

resource "aws_ssm_parameter" "ecs-paramstore-assume-role" {
  name  = "/${var.cluster_name}-${var.aws_env}/ecs-deploy/PARAMSTORE_ASSUME_ROLE"
  type  = "String"
  value = var.paramstore_assume_role
  count = var.paramstore_assume_role == "" ? 0 : 1
}

# dynamodb config
resource "aws_ssm_parameter" "ecs-deploy-dynamodb" {
  name  = "/${var.cluster_name}-${var.aws_env}/ecs-deploy/DYNAMODB_TABLE"
  type  = "String"
  value = "ecs-deploy"
}

# service role
resource "aws_ssm_parameter" "ecs-deploy-service-role" {
  name  = "/${var.cluster_name}-${var.aws_env}/ecs-deploy/AWS_ECS_SERVICE_ROLE"
  type  = "String"
  value = aws_iam_role.cluster-service-role.name
}

# cloudwatch config
resource "aws_ssm_parameter" "ecs-deploy-paramstore-cloudwatch" {
  name  = "/${var.cluster_name}-${var.aws_env}/ecs-deploy/CLOUDWATCH_LOGS_ENABLED"
  type  = "String"
  value = "yes"
}

resource "aws_ssm_parameter" "ecs-deploy-paramstore-cloudwatch-prefix" {
  name  = "/${var.cluster_name}-${var.aws_env}/ecs-deploy/CLOUDWATCH_LOGS_PREFIX"
  type  = "String"
  value = var.cluster_name
}

# ALB config
resource "aws_ssm_parameter" "ecs-deploy-loadbalancer-domain" {
  name  = "/${var.cluster_name}-${var.aws_env}/ecs-deploy/LOADBALANCER_DOMAIN"
  type  = "String"
  value = var.cluster_domain
}

# Autoscaling strategies
resource "aws_ssm_parameter" "ecs-deploy-autoscaling-strategies" {
  name  = "/${var.cluster_name}-${var.aws_env}/ecs-deploy/AUTOSCALING_STRATEGIES"
  type  = "String"
  value = var.autoscaling_strategies
  count = var.autoscaling_strategies == "" ? 0 : 1
}

# Paramstore auto-inject env vars
resource "aws_ssm_parameter" "ecs-deploy-paramstore-auto-inject" {
  name  = "/${var.cluster_name}-${var.aws_env}/ecs-deploy/PARAMSTORE_INJECT"
  type  = "String"
  value = var.paramstore_inject
}

