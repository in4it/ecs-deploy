#
# ECS ami
#

data "aws_ami" "ecs" {
  most_recent = true

  filter {
    name   = "name"
    values = ["amzn2-ami-ecs-*"]
  }

  filter {
    name   = "architecture"
    values = ["x86_64"]
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
  name               = var.cluster_name
  capacity_providers = var.ecs_capacity_provider_enabled ? [aws_ecs_capacity_provider.deploy_arm64[0].name,aws_ecs_capacity_provider.deploy_x86_64[0].name] : []
  dynamic "default_capacity_provider_strategy" {
    for_each = var.ecs_capacity_provider_enabled ? [aws_ecs_capacity_provider.deploy_arm64[0],aws_ecs_capacity_provider.deploy_x86_64[0]] : []
    content {
      base              = 0
      capacity_provider = default_capacity_provider_strategy.value.name
      weight            = 1
    }
  }
}

data "template_file" "ecs_init" {
  template = file(
  var.ecs_init_script == "" ? "${path.module}/templates/ecs-init.sh" : var.ecs_init_script,
  )

  vars = {
    CLUSTER_NAME  = var.cluster_name
    YUM_PROXY_URL = var.yum_proxy_url
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
  user_data              = base64encode(data.template_file.ecs_init.rendered)

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

  dynamic "metadata_options" {
    for_each = length(var.metadata_options_http_tokens) == 0 ? [] : [1]
    content {
      http_endpoint               = "enabled"
      http_tokens                 = var.metadata_options_http_tokens
      http_put_response_hop_limit = 1
    }
  }
}

#
# autoscaling
#
resource "aws_autoscaling_group" "cluster_x86_64" {
  name                  = "ecs-${var.cluster_name}-autoscaling-x86-64"
  vpc_zone_identifier   = var.vpc_private_subnets
  termination_policies  = var.cluster_termination_policies
  min_size              = var.cluster_minsize
  max_size              = var.cluster_maxsize
  desired_capacity      = var.cluster_desired_capacity
  protect_from_scale_in = var.ecs_capacity_provider_enabled ? true : false

  launch_template {
    id      = aws_launch_template.cluster.id
    version = "$Latest"
  }

  tag {
    key                 = "Name"
    value               = "${var.cluster_name}-ecs-x86-64"
    propagate_at_launch = true
  }

  tag {
    key                 = "Cluster"
    value               = "${var.cluster_name}_X86_64"
    propagate_at_launch = true
  }
}

resource "aws_autoscaling_group" "cluster_arm64" {
  name                  = "ecs-${var.cluster_name}-autoscaling-arm64"
  vpc_zone_identifier   = var.vpc_private_subnets
  termination_policies  = var.cluster_termination_policies
  min_size              = var.cluster_minsize
  max_size              = var.cluster_maxsize
  desired_capacity      = var.cluster_desired_capacity
  protect_from_scale_in = var.ecs_capacity_provider_enabled ? true : false

  launch_template {
    id      = aws_launch_template.cluster.id
    version = "$Latest"
  }

  tag {
    key                 = "Name"
    value               = "${var.cluster_name}-ecs-arm64"
    propagate_at_launch = true
  }

  tag {
    key                 = "Cluster"
    value               = "${var.cluster_name}_ARM64"
    propagate_at_launch = true
  }
}

resource "aws_autoscaling_lifecycle_hook" "cluster_x86_64" {
  count                  = var.ecs_capacity_provider_enabled ? 0 : 1
  name                   = "${var.cluster_name}-hook-x86-64"
  autoscaling_group_name = aws_autoscaling_group.cluster_x86_64.name
  default_result         = "CONTINUE"
  heartbeat_timeout      = 900
  lifecycle_transition   = "autoscaling:EC2_INSTANCE_TERMINATING"
}


resource "aws_autoscaling_lifecycle_hook" "cluster_arm64" {
  count                  = var.ecs_capacity_provider_enabled ? 0 : 1
  name                   = "${var.cluster_name}-hook-arm64"
  autoscaling_group_name = aws_autoscaling_group.cluster_arm64.name
  default_result         = "CONTINUE"
  heartbeat_timeout      = 900
  lifecycle_transition   = "autoscaling:EC2_INSTANCE_TERMINATING"
}


resource "aws_ecs_capacity_provider" "deploy_x86_64" {
  count = var.ecs_capacity_provider_enabled ? 1 : 0
  name  = "deploy-x86-64"

  auto_scaling_group_provider {
    auto_scaling_group_arn         = aws_autoscaling_group.cluster_x86_64.arn
    managed_termination_protection = "ENABLED"

    managed_scaling {
      maximum_scaling_step_size = var.capacity_maximum_scaling_step_size
      minimum_scaling_step_size = var.capacity_minimum_scaling_step_size
      status                    = "ENABLED"
      target_capacity           = var.target_capacity
    }
  }
}

resource "aws_ecs_capacity_provider" "deploy_arm64" {
  count = var.ecs_capacity_provider_enabled ? 1 : 0
  name  = "deploy-arm64"

  auto_scaling_group_provider {
    auto_scaling_group_arn         = aws_autoscaling_group.cluster_arm64.arn
    managed_termination_protection = "ENABLED"

    managed_scaling {
      maximum_scaling_step_size = var.capacity_maximum_scaling_step_size
      minimum_scaling_step_size = var.capacity_minimum_scaling_step_size
      status                    = "ENABLED"
      target_capacity           = var.target_capacity
    }
  }
}

