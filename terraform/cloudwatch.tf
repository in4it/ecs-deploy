#
# Cloudwatch logs
#
resource "aws_cloudwatch_log_group" "cluster" {
  name = "${var.cluster_name}-${var.aws_env}"
}

