# ECS deploy
ECS Deploy is a REST API server written in Go that can be used to deploy services on ECS from anywhere. It typically is executed as part of your deployment pipeline. Continuous Integration software (like Jenkins, CircleCI, Bitbucket or others) often don't have proper integration with ECS. This API server can be deployed on ECS and will be used to provide continuous deployment on ECS.

* Registers services in DynamoDB
* Creates ECR repository
* Creates necessary IAM roles
* Creates ALB target and listener rules
* Creates and updates ECS Services based on JSON input
* SAML supported Web UI to redeploy/rollback versions, add/update/delete parameters, examine event/container logs, scale, and run manual tasks
* Support to scale out and scale in ECS Container Instances

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

Alternatively you can use terraform to deploy the ecs cluster. See examples/ecs-deploy.tf for a terraform example that spins up an ecs cluster. You will need to use the IAM policy from examples/iam-policy.json to give ecs-deploy the necessary permissions.

### Deploy to ECS Cluster

To deploy the examples (an nginx server and a echoserver), use ecs-client:

```
./ecs-client login --url http://yourdomain/ecs-cluster
./ecs-client deploy -f examples/services/multiple-services/multiple-services.json

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

* AWS\_ACCOUNT\_ENV=staging 
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

## License
Copyright 2017 in4it BVBA

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
