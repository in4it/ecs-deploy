#
# ssm key
#
data "aws_kms_key" "ssm" {
  key_id = var.create_kms_key == "true" ? aws_kms_key.ssm[0].arn : "alias/aws/ssm"
}

resource "aws_kms_key" "ssm" {
  description             = "${var.cluster_name} SSM key"
  deletion_window_in_days = 30
  count                   = var.create_kms_key == "true" ? 1 : 0
}

resource "aws_kms_alias" "ssm" {
  name          = "alias/ssm-${var.aws_env}"
  target_key_id = aws_kms_key.ssm[0].key_id
  count         = var.create_kms_key == "true" ? 1 : 0
}

