# Terraform module for ecs-deploy

# pre-requisites
* Create a VPC or use the default VPC
* Create a domain name to be used by the LoadBalancer
* Issue a certificate using the AWS Certificate Manager (use the same name for the certificate as the cluster\_domain property)

# Code

```
module "ecs-deploy" {
  source                   = "github.com/in4it/ecs-deploy//terraform"
  cluster_name             = "mycluster"
  cluster_domain           = "mydomain.com"
  alb_internal             = false
  create_kms_key           = "true" # create a new KMS key instead of using the default ssm key
  vpc_id                   = "vpc-123456"
  vpc_public_subnets       = ["subnet-123456", "subnet-123456"]
  vpc_private_subnets      = ["subnet-123456", "subnet-123456"]
  aws_region               = "us-east-1"
  aws_env                  = "prod"
  instance_type            = "t3.small"
  ssh_key_name             = "${aws_key_pair.mykey.key_name}"
  cluster_minsize          = "1"
  cluster_maxsize          = "1"
  cluster_desired_capacity = "1"
  paramstore_enabled       = "yes"
  ecs_capacity_provider_enabled = true
}

# ssh key
resource "aws_key_pair" "mykey" {
  key_name   = "mykey"
  public_key = "${file("mykey.pub")}"
}
```

# Set keys & passwords

* The secret keys are not set in terraform (they'd be kept in the state otherwise)
* You can manually add the secrets or use the commands below to populate the parameter store

```
aws ssm put-parameter --name '/mycluster-prod/ecs-deploy/JWT_SECRET' --type SecureString --value 'secret' --key-id 'arn:aws:kms:region:0123456789:key/key-id' --region region
aws ssm put-parameter --name '/mycluster-prod/ecs-deploy/DEPLOY_PASSWORD' --type SecureString --value 'secret' --key-id 'arn:aws:kms:region:0123456789:key/key-id' --region region
```

# More Configuration Options
| Variable | Description |
| -------- | ----------- |
| enable\_lb\_logs | Provide new logs from elb with true value  |
| bucket_lb_logs | Provide location of the bucket (.bucket) to the elb logs if enable\_lb\_logs is true |
| ecs\_init\_script | Provide new (local) path to the ecs init script |
| ecs\_ecs2\_extra\_sg | Provide extra security group for EC2 instance |
| ecs\_ecs2\_vpc\_cidr\_sg | Provide change egress CIDR in the cluster sg for EC2 instance |
| sns\_endpoint | Override sns endpoint domain |
| ecs_capacity_provider_enabled | Enable AWS ECS capacity provider |
| capacity_maximum_scaling_step_size | Capacity provider maximum scaling step |
| capacity_minimum_scaling_step_size | Capacity provider minimum scaling step |
| target_capacity | Target capacity |

## Capacity provider migrations notes:
Before applying ecs-deploy module with `ecs_capacity_provider_enabled` set to `true`, make sure that all instances in AWS ASG have `scale in protection` enabled, otherwise it will result in an error:

```
Error: error creating capacity provider: ClientException: The managed termination protection setting for the capacity provider is invalid. To enable managed termination protection for a capacity provider, the Auto Scaling group must have instance protection from scale in enabled.
```
