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

<!-- BEGIN_TF_DOCS -->
## Requirements

| Name | Version |
|------|---------|
| <a name="requirement_terraform"></a> [terraform](#requirement\_terraform) | >= 0.12 |

## Providers

| Name | Version |
|------|---------|
| <a name="provider_aws"></a> [aws](#provider\_aws) | n/a |

## Modules

No modules.

## Resources

| Name | Type |
|------|------|
| [aws_alb.alb](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/alb) | resource |
| [aws_alb_listener.alb-http](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/alb_listener) | resource |
| [aws_alb_listener.alb-https](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/alb_listener) | resource |
| [aws_alb_listener_rule.ecs-deploy](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/alb_listener_rule) | resource |
| [aws_alb_target_group.ecs-deploy](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/alb_target_group) | resource |
| [aws_appmesh_virtual_node.ecs-deploy](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/appmesh_virtual_node) | resource |
| [aws_appmesh_virtual_service.ecs-deploy](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/appmesh_virtual_service) | resource |
| [aws_autoscaling_group.cluster](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/autoscaling_group) | resource |
| [aws_autoscaling_lifecycle_hook.cluster](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/autoscaling_lifecycle_hook) | resource |
| [aws_cloudwatch_event_rule.ecs-deploy](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/cloudwatch_event_rule) | resource |
| [aws_cloudwatch_event_rule.ecs-deploy-autoscaling](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/cloudwatch_event_rule) | resource |
| [aws_cloudwatch_event_target.ecs-deploy](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/cloudwatch_event_target) | resource |
| [aws_cloudwatch_event_target.ecs-deploy-autoscaling](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/cloudwatch_event_target) | resource |
| [aws_cloudwatch_log_group.cluster](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/cloudwatch_log_group) | resource |
| [aws_cloudwatch_log_group.ecs-deploy](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/cloudwatch_log_group) | resource |
| [aws_dynamodb_table.ecs-deploy](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/dynamodb_table) | resource |
| [aws_ecs_capacity_provider.deploy](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/ecs_capacity_provider) | resource |
| [aws_ecs_cluster.cluster](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/ecs_cluster) | resource |
| [aws_ecs_cluster_capacity_providers.cluster](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/ecs_cluster_capacity_providers) | resource |
| [aws_ecs_service.ecs-deploy](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/ecs_service) | resource |
| [aws_ecs_task_definition.ecs-deploy](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/ecs_task_definition) | resource |
| [aws_ecs_task_definition.ecs-deploy-appmesh](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/ecs_task_definition) | resource |
| [aws_iam_instance_profile.cluster-ec2-role](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_instance_profile) | resource |
| [aws_iam_role.cluster-ec2-role](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_role) | resource |
| [aws_iam_role.cluster-service-role](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_role) | resource |
| [aws_iam_role.ecs-deploy](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_role) | resource |
| [aws_iam_role.ecs-task-execution-role](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_role) | resource |
| [aws_iam_role_policy.cluster-ec2-role](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_role_policy) | resource |
| [aws_iam_role_policy.cluster-service-role](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_role_policy) | resource |
| [aws_iam_role_policy.ecs-deploy-policy](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_role_policy) | resource |
| [aws_iam_role_policy.ecs-task-execution-role](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_role_policy) | resource |
| [aws_kms_alias.ssm](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/kms_alias) | resource |
| [aws_kms_key.ssm](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/kms_key) | resource |
| [aws_launch_template.cluster](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/launch_template) | resource |
| [aws_lb_listener_certificate.extra-certificates](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/lb_listener_certificate) | resource |
| [aws_security_group.alb](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/security_group) | resource |
| [aws_security_group.cluster](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/security_group) | resource |
| [aws_security_group.ecs-deploy-awsvpc](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/security_group) | resource |
| [aws_service_discovery_service.ecs-deploy](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/service_discovery_service) | resource |
| [aws_sns_topic.ecs-deploy](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/sns_topic) | resource |
| [aws_sns_topic_subscription.ecs-deploy](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/sns_topic_subscription) | resource |
| [aws_ssm_parameter.ecs-deploy-autoscaling-down-cooldown](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/ssm_parameter) | resource |
| [aws_ssm_parameter.ecs-deploy-autoscaling-down-interval](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/ssm_parameter) | resource |
| [aws_ssm_parameter.ecs-deploy-autoscaling-down-period](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/ssm_parameter) | resource |
| [aws_ssm_parameter.ecs-deploy-autoscaling-strategies](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/ssm_parameter) | resource |
| [aws_ssm_parameter.ecs-deploy-autoscaling-up-cooldown](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/ssm_parameter) | resource |
| [aws_ssm_parameter.ecs-deploy-autoscaling-up-interval](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/ssm_parameter) | resource |
| [aws_ssm_parameter.ecs-deploy-autoscaling-up-period](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/ssm_parameter) | resource |
| [aws_ssm_parameter.ecs-deploy-aws-account-env](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/ssm_parameter) | resource |
| [aws_ssm_parameter.ecs-deploy-dynamodb](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/ssm_parameter) | resource |
| [aws_ssm_parameter.ecs-deploy-kms-id](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/ssm_parameter) | resource |
| [aws_ssm_parameter.ecs-deploy-loadbalancer-domain](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/ssm_parameter) | resource |
| [aws_ssm_parameter.ecs-deploy-paramstore-auto-inject](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/ssm_parameter) | resource |
| [aws_ssm_parameter.ecs-deploy-paramstore-cloudwatch](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/ssm_parameter) | resource |
| [aws_ssm_parameter.ecs-deploy-paramstore-cloudwatch-prefix](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/ssm_parameter) | resource |
| [aws_ssm_parameter.ecs-deploy-paramstore-deploy-max-wait](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/ssm_parameter) | resource |
| [aws_ssm_parameter.ecs-deploy-paramstore-ecs-ecr-scan-on-push](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/ssm_parameter) | resource |
| [aws_ssm_parameter.ecs-deploy-paramstore-ip-whitelist](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/ssm_parameter) | resource |
| [aws_ssm_parameter.ecs-deploy-paramstore-prefix](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/ssm_parameter) | resource |
| [aws_ssm_parameter.ecs-deploy-saml-acs-url](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/ssm_parameter) | resource |
| [aws_ssm_parameter.ecs-deploy-saml-enabled](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/ssm_parameter) | resource |
| [aws_ssm_parameter.ecs-deploy-saml-metadata-url](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/ssm_parameter) | resource |
| [aws_ssm_parameter.ecs-deploy-service-role](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/ssm_parameter) | resource |
| [aws_ssm_parameter.ecs-deploy-url-prefix](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/ssm_parameter) | resource |
| [aws_ssm_parameter.ecs-paramstore-assume-role](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/ssm_parameter) | resource |
| [aws_acm_certificate.certificate](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/acm_certificate) | data source |
| [aws_acm_certificate.extra-certificates](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/acm_certificate) | data source |
| [aws_ami.ecs](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/ami) | data source |
| [aws_caller_identity.current](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/caller_identity) | data source |
| [aws_kms_key.ssm](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/kms_key) | data source |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_alb_internal"></a> [alb\_internal](#input\_alb\_internal) | true if ALB needs to be internal | `string` | `"false"` | no |
| <a name="input_autoscaling_down_cooldown"></a> [autoscaling\_down\_cooldown](#input\_autoscaling\_down\_cooldown) | autoscaling variables | `string` | `""` | no |
| <a name="input_autoscaling_down_interval"></a> [autoscaling\_down\_interval](#input\_autoscaling\_down\_interval) | n/a | `string` | `""` | no |
| <a name="input_autoscaling_down_period"></a> [autoscaling\_down\_period](#input\_autoscaling\_down\_period) | n/a | `string` | `""` | no |
| <a name="input_autoscaling_strategies"></a> [autoscaling\_strategies](#input\_autoscaling\_strategies) | enable/disable autoscaling strategies | `string` | `""` | no |
| <a name="input_autoscaling_up_cooldown"></a> [autoscaling\_up\_cooldown](#input\_autoscaling\_up\_cooldown) | n/a | `string` | `""` | no |
| <a name="input_autoscaling_up_interval"></a> [autoscaling\_up\_interval](#input\_autoscaling\_up\_interval) | n/a | `string` | `""` | no |
| <a name="input_autoscaling_up_period"></a> [autoscaling\_up\_period](#input\_autoscaling\_up\_period) | n/a | `string` | `""` | no |
| <a name="input_aws_env"></a> [aws\_env](#input\_aws\_env) | environment to use | `any` | n/a | yes |
| <a name="input_aws_region"></a> [aws\_region](#input\_aws\_region) | The AWS region to create things in. | `any` | n/a | yes |
| <a name="input_bucket_lb_logs"></a> [bucket\_lb\_logs](#input\_bucket\_lb\_logs) | Name bucket located alb logs if logs is true | `string` | `""` | no |
| <a name="input_capacity_maximum_scaling_step_size"></a> [capacity\_maximum\_scaling\_step\_size](#input\_capacity\_maximum\_scaling\_step\_size) | n/a | `number` | `1000` | no |
| <a name="input_capacity_minimum_scaling_step_size"></a> [capacity\_minimum\_scaling\_step\_size](#input\_capacity\_minimum\_scaling\_step\_size) | n/a | `number` | `1` | no |
| <a name="input_cloudwatch_log_group_kms_arn"></a> [cloudwatch\_log\_group\_kms\_arn](#input\_cloudwatch\_log\_group\_kms\_arn) | cloudwatch log group kms arn | `string` | `""` | no |
| <a name="input_cloudwatch_log_retention_period"></a> [cloudwatch\_log\_retention\_period](#input\_cloudwatch\_log\_retention\_period) | cloudwatch retention period in days | `string` | `"0"` | no |
| <a name="input_cluster_desired_capacity"></a> [cluster\_desired\_capacity](#input\_cluster\_desired\_capacity) | desired capacity of cluster | `any` | n/a | yes |
| <a name="input_cluster_domain"></a> [cluster\_domain](#input\_cluster\_domain) | Domain to use for ALB | `any` | n/a | yes |
| <a name="input_cluster_maxsize"></a> [cluster\_maxsize](#input\_cluster\_maxsize) | maximum size of cluster | `any` | n/a | yes |
| <a name="input_cluster_minsize"></a> [cluster\_minsize](#input\_cluster\_minsize) | minimum size of cluster | `any` | n/a | yes |
| <a name="input_cluster_name"></a> [cluster\_name](#input\_cluster\_name) | Cluster name | `string` | `"services"` | no |
| <a name="input_cluster_termination_policies"></a> [cluster\_termination\_policies](#input\_cluster\_termination\_policies) | Termination policies of cluster | `list` | <pre>[<br>  "OldestLaunchTemplate",<br>  "OldestInstance"<br>]</pre> | no |
| <a name="input_cpu_credits"></a> [cpu\_credits](#input\_cpu\_credits) | CPU credits type for launch template | `string` | `"standard"` | no |
| <a name="input_create_kms_key"></a> [create\_kms\_key](#input\_create\_kms\_key) | Create a KMS key for ssm or use default ssm key | `any` | n/a | yes |
| <a name="input_drop_invalid_header_fields"></a> [drop\_invalid\_header\_fields](#input\_drop\_invalid\_header\_fields) | true if needs to drop invalid header fields | `string` | `"false"` | no |
| <a name="input_dynamodb_pitr_recovery_period"></a> [dynamodb\_pitr\_recovery\_period](#input\_dynamodb\_pitr\_recovery\_period) | number of days to retain recovery points for | `number` | `35` | no |
| <a name="input_dynamodb_read_capacity"></a> [dynamodb\_read\_capacity](#input\_dynamodb\_read\_capacity) | n/a | `number` | `2` | no |
| <a name="input_dynamodb_write_capacity"></a> [dynamodb\_write\_capacity](#input\_dynamodb\_write\_capacity) | n/a | `number` | `2` | no |
| <a name="input_ecs_capacity_provider_enabled"></a> [ecs\_capacity\_provider\_enabled](#input\_ecs\_capacity\_provider\_enabled) | n/a | `bool` | `false` | no |
| <a name="input_ecs_deploy_appmesh_name"></a> [ecs\_deploy\_appmesh\_name](#input\_ecs\_deploy\_appmesh\_name) | appmesh name | `string` | `""` | no |
| <a name="input_ecs_deploy_appmesh_release"></a> [ecs\_deploy\_appmesh\_release](#input\_ecs\_deploy\_appmesh\_release) | appmesh release version | `string` | `"v1.11.1.1-prod"` | no |
| <a name="input_ecs_deploy_awsvpc"></a> [ecs\_deploy\_awsvpc](#input\_ecs\_deploy\_awsvpc) | enable awsvpc for the ecs-deploy ecs service | `bool` | `false` | no |
| <a name="input_ecs_deploy_awsvpc_allowsg"></a> [ecs\_deploy\_awsvpc\_allowsg](#input\_ecs\_deploy\_awsvpc\_allowsg) | allow extra sgs when using awsvpc | `string` | `""` | no |
| <a name="input_ecs_deploy_cpu"></a> [ecs\_deploy\_cpu](#input\_ecs\_deploy\_cpu) | n/a | `number` | `128` | no |
| <a name="input_ecs_deploy_debug"></a> [ecs\_deploy\_debug](#input\_ecs\_deploy\_debug) | ecs deploy debug | `string` | `"false"` | no |
| <a name="input_ecs_deploy_enable_appmesh"></a> [ecs\_deploy\_enable\_appmesh](#input\_ecs\_deploy\_enable\_appmesh) | enable appmesh | `bool` | `false` | no |
| <a name="input_ecs_deploy_image"></a> [ecs\_deploy\_image](#input\_ecs\_deploy\_image) | image location of ecs-deploy | `string` | `"709825985650.dkr.ecr.us-east-1.amazonaws.com/in4it/ecs-deploy"` | no |
| <a name="input_ecs_deploy_max_wait_seconds"></a> [ecs\_deploy\_max\_wait\_seconds](#input\_ecs\_deploy\_max\_wait\_seconds) | n/a | `number` | `900` | no |
| <a name="input_ecs_deploy_memory_reservation"></a> [ecs\_deploy\_memory\_reservation](#input\_ecs\_deploy\_memory\_reservation) | n/a | `number` | `64` | no |
| <a name="input_ecs_deploy_service_discovery_domain"></a> [ecs\_deploy\_service\_discovery\_domain](#input\_ecs\_deploy\_service\_discovery\_domain) | service discovery domain | `string` | `""` | no |
| <a name="input_ecs_deploy_service_discovery_id"></a> [ecs\_deploy\_service\_discovery\_id](#input\_ecs\_deploy\_service\_discovery\_id) | join a service discovery domain providing the id | `string` | `""` | no |
| <a name="input_ecs_deploy_version"></a> [ecs\_deploy\_version](#input\_ecs\_deploy\_version) | ecs deploy version | `string` | `"v1.0.43"` | no |
| <a name="input_ecs_ec2_extra_sg"></a> [ecs\_ec2\_extra\_sg](#input\_ecs\_ec2\_extra\_sg) | n/a | `string` | `""` | no |
| <a name="input_ecs_ec2_vpc_cidr_sg"></a> [ecs\_ec2\_vpc\_cidr\_sg](#input\_ecs\_ec2\_vpc\_cidr\_sg) | n/a | `string` | `"0.0.0.0/0"` | no |
| <a name="input_ecs_ecr_scan_on_push"></a> [ecs\_ecr\_scan\_on\_push](#input\_ecs\_ecr\_scan\_on\_push) | n/a | `string` | `"false"` | no |
| <a name="input_ecs_init_script"></a> [ecs\_init\_script](#input\_ecs\_init\_script) | n/a | `string` | `""` | no |
| <a name="input_ecs_whitelist"></a> [ecs\_whitelist](#input\_ecs\_whitelist) | n/a | `string` | `"0.0.0.0/0"` | no |
| <a name="input_enable_dynamodb_encryption"></a> [enable\_dynamodb\_encryption](#input\_enable\_dynamodb\_encryption) | DynamoDB variables | `bool` | `false` | no |
| <a name="input_enable_dynamodb_pitr"></a> [enable\_dynamodb\_pitr](#input\_enable\_dynamodb\_pitr) | enable point in time recovery | `bool` | `false` | no |
| <a name="input_enable_lb_logs"></a> [enable\_lb\_logs](#input\_enable\_lb\_logs) | true if needs logs for ALB | `string` | `"false"` | no |
| <a name="input_extra_domains"></a> [extra\_domains](#input\_extra\_domains) | extra domain that need to be supported by the ALB | `list` | `[]` | no |
| <a name="input_fixed_response_body"></a> [fixed\_response\_body](#input\_fixed\_response\_body) | fixed response body | `string` | `"No service configured at this address"` | no |
| <a name="input_fixed_response_code"></a> [fixed\_response\_code](#input\_fixed\_response\_code) | fixed response http code | `string` | `"404"` | no |
| <a name="input_fixed_response_content_type"></a> [fixed\_response\_content\_type](#input\_fixed\_response\_content\_type) | fixed response content type | `string` | `"text/plain"` | no |
| <a name="input_instance_type"></a> [instance\_type](#input\_instance\_type) | instance type | `any` | n/a | yes |
| <a name="input_metadata_options_http_tokens"></a> [metadata\_options\_http\_tokens](#input\_metadata\_options\_http\_tokens) | metadata options IMDSv1 or IMDSv2 | `string` | `"required"` | no |
| <a name="input_paramstore_assume_role"></a> [paramstore\_assume\_role](#input\_paramstore\_assume\_role) | assume role when using paramstore | `string` | `""` | no |
| <a name="input_paramstore_enabled"></a> [paramstore\_enabled](#input\_paramstore\_enabled) | Enable parameter store | `any` | n/a | yes |
| <a name="input_paramstore_inject"></a> [paramstore\_inject](#input\_paramstore\_inject) | n/a | `string` | `"no"` | no |
| <a name="input_prod_code"></a> [prod\_code](#input\_prod\_code) | n/a | `string` | `"3x0v7m3npdgzaiw2f8lwsgju5"` | no |
| <a name="input_saml_acs_url"></a> [saml\_acs\_url](#input\_saml\_acs\_url) | saml acs url, if the default acs url needs to be overwritten | `string` | `""` | no |
| <a name="input_saml_enabled"></a> [saml\_enabled](#input\_saml\_enabled) | Enable SAML auth | `string` | `"no"` | no |
| <a name="input_saml_metadata_url"></a> [saml\_metadata\_url](#input\_saml\_metadata\_url) | SAML metadata url | `string` | `"https://identityprovider/metadata.xml"` | no |
| <a name="input_sns_endpoint"></a> [sns\_endpoint](#input\_sns\_endpoint) | sns variables | `string` | `""` | no |
| <a name="input_sns_kms_master_key_id"></a> [sns\_kms\_master\_key\_id](#input\_sns\_kms\_master\_key\_id) | KMS key arn to encrypt SNS topic | `string` | `""` | no |
| <a name="input_ssh_key_name"></a> [ssh\_key\_name](#input\_ssh\_key\_name) | ssh key name | `any` | n/a | yes |
| <a name="input_ssl_policy"></a> [ssl\_policy](#input\_ssl\_policy) | TLS policy for https listener | `string` | `"ELBSecurityPolicy-TLS13-1-2-2021-06"` | no |
| <a name="input_target_capacity"></a> [target\_capacity](#input\_target\_capacity) | n/a | `number` | `100` | no |
| <a name="input_url_prefix"></a> [url\_prefix](#input\_url\_prefix) | URL prefix | `string` | `"/ecs-deploy"` | no |
| <a name="input_volume_type"></a> [volume\_type](#input\_volume\_type) | volume type | `string` | `"gp2"` | no |
| <a name="input_vpc_id"></a> [vpc\_id](#input\_vpc\_id) | VPC ID | `any` | n/a | yes |
| <a name="input_vpc_private_subnets"></a> [vpc\_private\_subnets](#input\_vpc\_private\_subnets) | VPC private subnets | `list(string)` | n/a | yes |
| <a name="input_vpc_public_subnets"></a> [vpc\_public\_subnets](#input\_vpc\_public\_subnets) | VPC public subnets | `list(string)` | n/a | yes |
| <a name="input_yum_proxy_url"></a> [yum\_proxy\_url](#input\_yum\_proxy\_url) | yum http proxy url | `string` | `""` | no |

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_alb-dns-name"></a> [alb-dns-name](#output\_alb-dns-name) | n/a |
| <a name="output_alb-sg"></a> [alb-sg](#output\_alb-sg) | n/a |
| <a name="output_alb-zone-id"></a> [alb-zone-id](#output\_alb-zone-id) | n/a |
| <a name="output_cluster-cloudwatch-log-group-name"></a> [cluster-cloudwatch-log-group-name](#output\_cluster-cloudwatch-log-group-name) | n/a |
| <a name="output_cluster-ec2-role-arn"></a> [cluster-ec2-role-arn](#output\_cluster-ec2-role-arn) | n/a |
| <a name="output_cluster-ec2-role-name"></a> [cluster-ec2-role-name](#output\_cluster-ec2-role-name) | n/a |
| <a name="output_ecs-deploy-cloudwatch-log-group-name"></a> [ecs-deploy-cloudwatch-log-group-name](#output\_ecs-deploy-cloudwatch-log-group-name) | n/a |
| <a name="output_paramstore-kms-arn"></a> [paramstore-kms-arn](#output\_paramstore-kms-arn) | n/a |
| <a name="output_sg-cluster"></a> [sg-cluster](#output\_sg-cluster) | n/a |
<!-- END_TF_DOCS -->