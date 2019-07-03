resource "aws_service_discovery_service" "ecs-deploy" {
  count = var.ecs_deploy_service_discovery_id == "" ? 0 : 1
  name = "ecs-deploy"

  dns_config {
    namespace_id = var.ecs_deploy_service_discovery_id

    dns_records {
      ttl  = 30
      type = "A"
    }

    routing_policy = "MULTIVALUE"
  }

  health_check_custom_config {
    failure_threshold = 1
  }
}