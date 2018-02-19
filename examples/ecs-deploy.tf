#
# ecs-deploy example
#

#
# variables
#
variable CLUSTER_ID {}

variable VPC_ID {}
variable CLUSTER_SG {}
variable SUBNET_1 {}
variable SUBNET_2 {}
variable DOMAIN {}

# 
# ECS cluster (imported)
# 

data "aws_ecs_cluster" "cluster" {
  cluster_name = "${CLUSTER_NAME}"
}

#
# ALB
#

resource "aws_alb" "alb" {
  name            = "${data.aws_ecs_cluster.cluster.name}"
  internal        = false
  security_groups = ["${var.CLUSTER_SG}"]
  subnets         = ["${var.SUBNET_1}", "${var.SUBNET_2}"]

  enable_deletion_protection = true
}

data "aws_acm_certificate" "certificate" {
  domain   = "${var.DOMAIN}"
  statuses = ["ISSUED"]
}

resource "aws_alb_listener" "listener" {
  load_balancer_arn = "${aws_alb.alb.arn}"
  port              = "443"
  protocol          = "HTTPS"
  ssl_policy        = "ELBSecurityPolicy-2015-05"
  certificate_arn   = "${data.aws_acm_certificate.certificate.arn}"

  default_action {
    target_group_arn = "${aws_alb_target_group.ecs-deploy.arn}"
    type             = "forward"
  }
}

resource "aws_alb_target_group" "ecs-deploy" {
  name                 = "ecs-deploy"
  port                 = 8080
  protocol             = "HTTP"
  vpc_id               = "${var.VPC_ID}"
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

#
# ECS service
#

resource "aws_ecs_service" "ecs-deploy" {
  name                               = "ecs-deploy"
  cluster                            = "${data.aws_ecs_cluster.cluster.id}"
  task_definition                    = "${aws_ecs_task_definition.ecs-deploy.arn}"
  iam_role                           = "${aws_iam_role.ecs_service_role.arn}"
  desired_count                      = 1
  deployment_minimum_healthy_percent = 100
  deployment_maximum_percent         = 200

  load_balancer {
    target_group_arn = "${aws_alb_target_group.ecs-deploy.id}"
    container_name   = "ecs-deploy"
    container_port   = 8080
  }
}

data "template_file" "ecs-deploy" {
  template = "${file("ecs-deploy.json")}"
}

resource "aws_ecs_task_definition" "ecs-deploy" {
  family                = "ecs-deploy"
  container_definitions = "${data.template_file.ecs-deploy.rendered}"
  task_role_arn         = "${aws_iam_role.ecs-deploy.arn}"
}

#
# IAM role & policy
#
resource "aws_iam_role" "ecs-deploy" {
  name = "ecs-deploy"

  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": "sts:AssumeRole",
      "Principal": {
        "Service": "ecs-tasks.amazonaws.com"
      },
      "Effect": "Allow",
      "Sid": ""
    }
  ]
}
EOF
}

resource "aws_iam_role_policy" "ecs-deploy-policy" {
  name   = "ecs-deploy-policy"
  role   = "${aws_iam_role.ecs-deploy.id}"
  policy = "${file("iam-policy.json")}"
}

#
# dynamodb
#
resource "aws_dynamodb_table" "ecs-deploy-services" {
  name           = "Services"
  read_capacity  = 5
  write_capacity = 5
  hash_key       = "ServiceName"
  range_key      = "Time"

  attribute {
    name = "ServiceName"
    type = "S"
  }

  attribute {
    name = "Time"
    type = "S"
  }

  attribute {
    name = "Day"
    type = "S"
  }

  attribute {
    name = "Month"
    type = "S"
  }

  ttl {
    attribute_name = "ExpirationTimeTTL"
    enabled        = true
  }

  global_secondary_index {
    name            = "DayIndex"
    hash_key        = "Day"
    range_key       = "Time"
    write_capacity  = 5
    read_capacity   = 5
    projection_type = "ALL"
  }

  global_secondary_index {
    name            = "MonthIndex"
    hash_key        = "Month"
    range_key       = "Time"
    write_capacity  = 5
    read_capacity   = 5
    projection_type = "ALL"
  }
}
