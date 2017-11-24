resource "aws_alb_target_group" "${SERVICE}" {
  # arn = ${TARGET_GROUP_ARN}
  name     = "${SERVICE}"
  port     = ${SERVICE_PORT}
  protocol = "${SERVICE_PROTOCOL}"
  vpc_id   = "${VPC_ID}"
  deregistration_delay = 300
  ${HEALTHCHECK}
}
