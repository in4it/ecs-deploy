#
# parameter store configuration
#
resource "aws_ssm_parameter" "prefix" {
  name  = "/${var.cluster-name}-${var.aws_env}/ecs-deploy/URL_PREFIX"
  type  = "String"
  value = "/ecs-deploy"
}

resource "aws_ssm_parameter" "kms-arn" {
  name  = "/${var.cluster-name}-${var.aws_env}/ecs-deploy/PARAMSTORE_KMS_ARN"
  type  = "String"
  value = "${data.aws_kms_key.ssm.arn}"
}
