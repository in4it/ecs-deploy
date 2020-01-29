resource "aws_appmesh_virtual_node" "ecs-deploy" {
  count     = var.ecs_deploy_enable_appmesh ? 1 : 0
  name      = "ecs-deploy"
  mesh_name = var.ecs_deploy_appmesh_name

  spec {
    listener {
      port_mapping {
        port     = 8080
        protocol = "http"
      }

      health_check {
        protocol            = "http"
        path                = "/ecs-deploy/health"
        healthy_threshold   = 2
        unhealthy_threshold = 2
        timeout_millis      = 2000
        interval_millis     = 30000
      }
    }

    service_discovery {
      dns {
        hostname = "ecs-deploy.${var.ecs_deploy_service_discovery_domain}"
      }
    }
  }
}
resource "aws_appmesh_virtual_service" "ecs-deploy" {
  count     = var.ecs_deploy_enable_appmesh ? 1 : 0
  name      = "ecs-deploy.${var.ecs_deploy_service_discovery_domain}"
  mesh_name = var.ecs_deploy_appmesh_name

  spec {
    provider {
      virtual_node {
        virtual_node_name = aws_appmesh_virtual_node.ecs-deploy[0].name
      }
    }
  }
}
