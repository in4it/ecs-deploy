resource "aws_security_group" "alb" {
  name        = "${var.cluster_name} ALB"
  vpc_id      = var.vpc_id
  description = "${var.cluster_name} ALB"

  ingress {
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_security_group" "cluster" {
  name        = "${var.cluster_name} ECS"
  vpc_id      = var.vpc_id
  description = "${var.cluster_name} ECS"

  ingress {
    from_port       = 32768
    to_port         = 61000
    protocol        = "tcp"
    security_groups = [aws_security_group.alb.id]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}


resource "aws_security_group" "ecs-deploy-awsvpc" {
  name        = "${var.cluster_name} ECS - ecs-deploy-awsvpc"
  vpc_id      = var.vpc_id
  description = "${var.cluster_name} ECS - ecs-deploy-awsvpc"

  ingress {
    from_port       = 8080
    to_port         = 8080
    protocol        = "tcp"
    security_groups = compact(
    split(
      ",",
      format("%s,%s", aws_security_group.alb.id, var.ecs_deploy_awsvpc_allowsg),
    ),
  )
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}


