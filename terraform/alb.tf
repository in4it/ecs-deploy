# 
# ECS ALB
#

data "aws_acm_certificate" "certificate" {
  domain   = var.cluster_domain
  statuses = ["ISSUED"]
}

resource "aws_alb" "alb" {
  name            = "${var.cluster_name}${var.alb_internal == "true" ? "-private" : ""}"
  internal        = var.alb_internal
  security_groups = [aws_security_group.alb.id]
  subnets = formatlist(
    var.alb_internal == "true" ? "%[1]s" : "%[2]s",
    var.vpc_private_subnets,
    var.vpc_public_subnets,
  )

  enable_deletion_protection = true
}

resource "aws_alb_target_group" "ecs-deploy" {
  name                 = "ecs-deploy"
  port                 = 8080
  protocol             = "HTTP"
  target_type          = var.ecs_deploy_awsvpc ? "ip" : "instance"
  vpc_id               = var.vpc_id
  deregistration_delay = 30

  health_check {
    healthy_threshold   = 3
    unhealthy_threshold = 3
    protocol            = "HTTP"
    path                = "/ecs-deploy/health"
    interval            = 60
    matcher             = "200"
  }
}

# rule for current lb
resource "aws_alb_listener_rule" "ecs-deploy" {
  listener_arn = aws_alb_listener.alb-https.arn
  priority     = 200

  action {
    type             = "forward"
    target_group_arn = aws_alb_target_group.ecs-deploy.arn
  }

  condition {
    field  = "path-pattern"
    values = ["/ecs-deploy/*"]
  }
}

# alb listener (https)
resource "aws_alb_listener" "alb-https" {
  load_balancer_arn = aws_alb.alb.arn
  port              = "443"
  protocol          = "HTTPS"
  ssl_policy        = "ELBSecurityPolicy-2016-08"
  certificate_arn   = data.aws_acm_certificate.certificate.arn

  default_action {
    type = "fixed-response"
    fixed_response {
      content_type = var.fixed_response_content_type
      message_body = var.fixed_response_body
      status_code  = var.fixed_response_code
    }
  }
}

# alb listener (http)
resource "aws_alb_listener" "alb-http" {
  load_balancer_arn = aws_alb.alb.arn
  port              = "80"
  protocol          = "HTTP"

  default_action {
    type = "fixed-response"
    fixed_response {
      content_type = var.fixed_response_content_type
      message_body = var.fixed_response_body
      status_code  = var.fixed_response_code
    }
  }
}

