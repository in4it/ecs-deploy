#
# ECS ami
#

data "aws_ami" "ecs" {
  most_recent = true

  filter {
    name   = "name"
    values = ["amzn-ami-*-amazon-ecs-optimized"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }

  owners = ["591542846629"] # AWS
}

#
# ECS cluster
#

resource "aws_ecs_cluster" "cluster" {
  name = var.cluster_name
}

data "template_file" "ecs_init" {
  template = file(
    var.ecs_init_script == "" ? "${path.module}/templates/ecs-init.sh" : var.ecs_init_script,
  )

  vars = {
    CLUSTER_NAME = var.cluster_name
  }
}

#
# launch template
#
resource "aws_launch_template" "cluster" {
  name                                 = "ecs-${var.cluster_name}-launchtemplate"
  image_id                             = data.aws_ami.ecs.id
  instance_type                        = var.instance_type
  key_name                             = var.ssh_key_name
  instance_initiated_shutdown_behavior = "terminate"

  iam_instance_profile {
    name = aws_iam_instance_profile.cluster-ec2-role.id
  }

  vpc_security_group_ids = compact(
    split(
      ",",
      format("%s,%s", aws_security_group.cluster.id, var.ecs_ec2_extra_sg),
    ),
  )
  user_data = base64encode(data.template_file.ecs_init.rendered)

  credit_specification {
    cpu_credits = var.cpu_credits
  }

  tag_specifications {
    resource_type = "instance"

    tags = {
      Name = "${var.cluster_name}-ecs"
    }
  }

  lifecycle {
    create_before_destroy = true
  }
}

#
# autoscaling
#
resource "aws_autoscaling_group" "cluster" {
  name                 = "ecs-${var.cluster_name}-autoscaling"
  vpc_zone_identifier  = var.vpc_private_subnets
  termination_policies = var.cluster_termination_policies
  min_size             = var.cluster_minsize
  max_size             = var.cluster_maxsize
  desired_capacity     = var.cluster_desired_capacity

  launch_template {
    id      = aws_launch_template.cluster.id
    version = "$Latest"
  }

  tag {
    key                 = "Name"
    value               = "${var.cluster_name}-ecs"
    propagate_at_launch = true
  }

  tag {
    key                 = "Cluster"
    value               = var.cluster_name
    propagate_at_launch = true
  }
}

resource "aws_autoscaling_lifecycle_hook" "cluster" {
  name                   = "${var.cluster_name}-hook"
  autoscaling_group_name = aws_autoscaling_group.cluster.name
  default_result         = "CONTINUE"
  heartbeat_timeout      = 900
  lifecycle_transition   = "autoscaling:EC2_INSTANCE_TERMINATING"
}

