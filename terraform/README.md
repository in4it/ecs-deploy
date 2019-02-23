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
| ecs\_init\_script | Provide new (local) path to the ecs init script |
| ecs\_ecs2\_extra\_sg | Provide extra security group for EC2 instance |
| sns\_endpoint | Override sns endpoint domain |
