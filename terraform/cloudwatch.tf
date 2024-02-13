#
# Cloudwatch logs
#
resource "aws_cloudwatch_log_group" "cluster" {
  name              = "${var.cluster_name}-${var.aws_env}"
  kms_key_id        = var.cloudwatch_log_group_kms_arn
  retention_in_days = var.cloudwatch_log_retention_period
}
