data "aws_iam_role" "${SERVICE}-service-role" {
  name = "ecs-service-role"
}
data "aws_ecs_cluster" "${SERVICE}-ecs-cluster" {
  cluster_name = "${CLUSTERNAME}"
}
data "aws_ecs_task_definition" "${SERVICE}" {
  task_definition = "${SERVICE}"
}
# no terraform import available yet for aws_ecs_service
#resource "aws_ecs_service" "${SERVICE}" {
#  name = "${SERVICE}"
#  cluster = "${data.aws_ecs_cluster.${SERVICE}-ecs-cluster.id}"
#  task_definition = "${data.aws_ecs_task_definition.${SERVICE}.arn}"
#  iam_role = "${data.aws_iam_role.${SERVICE}-service-role.arn}"
#  desired_count = ${SERVICE_DESIREDCOUNT}
#  ${SERVICE_MINIMUMHEALTHYPERCENT}
#  ${SERVICE_MAXIMUMPERCENT}
#
#  load_balancer {
#    target_group_arn = "${aws_alb_target_group.${SERVICE}.id}"
#    container_name = "${SERVICE}"
#    container_port = ${SERVICE_PORT}
#  }
#}
