# ECS deploy
ECS Deploy is a REST API server written in Go that can be used to deploy services on ECS from anywhere. It typically is executed as part of your deployment pipeline. Continuous Integration software (like Jenkins, CircleCI, Bitbucket or others) often don't have proper integration with ECS. This API server can be deployed on ECS and will be used to provide continuous deployment on ECS.

* Registers services in DynamoDB
* Creates ECR repository
* Creates necessary IAM roles
* Creates ALB target and listener rules
* Creates and updates ECS Services based on json/yaml input
* SAML supported Web UI to redeploy/rollback versions, add/update/delete parameters, examine event/container logs, scale, and run manual tasks
* Support to scale out and scale in ECS Container Instances

## The UI

<p align="center">
  <a href="https://d3jb1lt6v0nddd.cloudfront.net/ecs-deploy/ecs-deploy-ui.gif">
    <img src="https://d3jb1lt6v0nddd.cloudfront.net/ecs-deploy/ecs-deploy-ui.gif" />
  </a>
</p>

## Usage

### Download

You can download ecs-deploy and ecs-client from the [releases page](https://github.com/in4it/ecs-deploy/releases) or you can use the [image from dockerhub](https://hub.docker.com/r/in4it/ecs-deploy/).

### Bootstrap ECS cluster

You can bootstrap a new ECS cluster using ecs-deploy. It'll setup a autoscaling group, ALB, IAM roles, and the ECS cluster.

```
./ecs-deploy --bootstrap \
  --alb-security-groups sg-123456 \
  --cloudwatch-logs-enabled \
  --cloudwatch-logs-prefix mycompany \
  --cluster-name mycluster \
  --ecs-desired-size 1 \
  --ecs-max-size 1 \
  --ecs-min-size 1 \
  --ecs-security-groups sg-123456 \
  --ecs-subnets subnet-123456 \
  --environment staging \
  --instance-type t2.micro \
  --key-name mykey \
  --loadbalancer-domain cluster.in4it.io \
  --paramstore-enabled \
  --paramstore-kms-arn aws:arn:kms:region:accountid:key/1234 \
  --paramstore-prefix mycompany \
  --profile your-aws-profile \
  --region your-aws-region
```

You'll need to setup the security groups and VPC/subnets first. The ALB security group should allow port 80 and 443 incoming, the ECS security group should allow 32768:61000 from the ALB.

If you no longer need the cluster, you can remove it by specifying --delete-cluster instead of --bootstrap

### Bootstrap with terraform
Alternatively you can use terraform to deploy the ecs cluster. See [terraform/README.md](https://github.com/in4it/ecs-deploy/blob/master/terraform/README.md) for a terraform module that spins up an ecs cluster.

### Deploy to ECS Cluster

To deploy the examples (an nginx server and a echoserver), use ecs-client:

Login interactively:
```
./ecs-client login --url http://yourdomain/ecs-cluster
```

Login with environment variables:
```
ECS_DEPLOY_LOGIN=deploy ECS_DEPLOY_PASSWORD=password ./ecs-client login --url http://yourdomain/ecs-cluster
```

Deploy:
```
./ecs-client deploy -f examples/services/multiple-services/multiple-services.yaml
```


## Configuration (Environment variables)

### AWS Specific variables:

* AWS\_REGION=region                  # mandatory

### Authentication variables;
* JWT\_SECRET=secret                   # mandatory
* DEPLOY\_PASSWORD=deploy              # mandatory
* DEVELOPER\_PASSWORD=developer        # mandatory

### Service specific variables 
These will be used when deploying services

* AWS\_ACCOUNT\_ENV=dev|staging|testing|qa|prod
* PARAMSTORE\_ENABLED=yes
* PARAMSTORE\_PREFIX=mycompany 
* PARAMSTORE\_KMS\_ARN=
* CLOUDWATCH\_LOGS\_ENABLED=yes
* CLOUDWATCH\_LOGS\_PREFIX=mycompany
* LOADBALANCER\_DOMAIN=mycompany.com

### DynamoDB specific variables
* DYNAMODB\_TABLE=Services

### SAML

SAML can be enabled using the following environment variables
* SAML\_ENABLED=yes
* SAML\_ACS\_URL=https://mycompany.com/url-prefix
* SAML\_CERTIFICATE=contents of your certificate
* SAML\_PRIVATE\_KEY=contents of your private key
* SAML\_METADATA\_URL=https://identity-provider/metadata.xml

To create a new key and certificate, the following openssl command can be used:
```
openssl req -x509 -newkey rsa:2048 -keyout myservice.key -out myservice.cert -days 3650 -nodes -subj "/CN=myservice.mycompany.com"
```

# Web UI

* PARAMSTORE\_ASSUME\_ROLE=arn # arn to assume when querying the parameter store

# Autoscaling (down and up)

## Setup

* Create an SNS topic, add https subscriber with URL https://your-domain.com/ecs-deploy/webhook
* Create a [CloudWatch Event for ECS tasks/services](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/cloudwatch_event_stream.html)
* Create an [EC2 Auto Scaling Lifecycle hook](https://docs.aws.amazon.com/autoscaling/ec2/userguide/lifecycle-hooks.html), and a CloudWatch event to capture the Lifecycle hook
* Let the SNS topic be the trigger for the CloudWatch events

## Usage

* Autoscaling (up) will be triggered when the largest container (in respect to mem/cpu) cannot be scheduled on the cluster
* Autoscaling (down) will be triggered when there is enough capacity available on the cluster to remove an instance (instance size + largest container + buffer)

## Configuration

The defaults are set for the most common use cases, but can be changed by setting environment variables:

| Environment variable       | Default value | Description |
| ---------------------      | ------------- | ----------- |
| PARAMSTORE\_ENABLED | no | Use "yes" to enable the parameter store. |
| PARAMSTORE\_PREFIX | "" | Prefix to use for the parameter store. mycompany will result in /mycompany/servicename/variable | 
| PARAMSTORE\_KMS\_ARN | "" | Specify a KMS ARN to encrypt/decrypt variables |
| PARAMSTORE\_INJECT | no | Use "Yes" to enable injection of secrets into the task definition |
| AUTOSCALING\_STRATEGIES  | LargestContainerUp,LargestContainerDown | List of autoscaling strategies to apply. See below for different types |
| AUTOSCALING\_DOWN\_STRATEGY  | gracefully | Only gracefully supported now (uses interval and period before executing the scaling down operation) |
| AUTOSCALING\_UP\_STRATEGY  | immediately | Scale up strategy  (immediatey, gracefully) |
| AUTOSCALING\_DOWN\_COOLDOWN | 5 | Cooldown period after scaling down |
| AUTOSCALING\_DOWN\_INTERVAL | 60 | Seconds between intervals to check resource usage before scaling, after a scaling down operation is detected |
| AUTOSCALING\_DOWN\_PERIOD | 5 | Periods to check before scaling |
| AUTOSCALING\_UP\_COOLDOWN | 5 | Cooldown period after scaling up |
| AUTOSCALING\_UP\_INTERVAL | 60 | Seconds between intervals to check resource usage before scaling, after a scaling up operation is detected |
| AUTOSCALING\_UP\_PERIOD | 5 | Periods to check before scaling |
| SERVICE\_DISCOVERY\_TTL | 60 | TTL for service discovery records |
| SERVICE_DISCOVERY_FAILURETHRESHOLD | 3 | Failure threshold for service discovery records |
| AWS\_RESOURCE\_CREATION\_ENABLED | yes | Let ecs-deploy create AWS IAM resources for you |
| SLACK\_WEBHOOKS | "" | Comma seperated Slack webhooks, optionally with a channel (format: url1:#channel,url2:#channel) |
| SLACK\_USERNAME | ecs-deploy | Slack username |


### Autoscaling Strategies

| Strategy       | Description |
| ---------------| ----------- |
| LargestContainerUp | Scale when the largest container (+buffer) in the cluster cannot be scheduled anymore on a node |
| LargestContainerDown | Scale down when there is enough capacity to schedule the largest container (buffer) after a node is removed |
| Polling | Poll all services every minute to check if a task can't be scheduled due to resource constraints (10 services per api call, only 1 call per second) |
