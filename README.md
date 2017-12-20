# ECS deploy
ECS Deploy is a REST API server written in Go that can be used to deploy services on ECS from anywhere. It typically is executed as part of your deployment pipeline. Continuous Integration software (like Jenkins, CircleCI, Bitbucket or others) often don't have proper integration with ECS. This API server can be deployed on ECS and will be used to provide continuous deployment on ECS.

* Registers services in DynamoDB
* Creates ECR repository
* Creates necessary IAM roles
* Creates ALB target and listener rules
* Creates and updates ECS Services based on JSON input

## How to deploy

* Deploy the docker image as a service on ECS
  * See examples/ecs-deploy.tf for a terraform deploy script
    * This script creates an ALB with the same name as the ECS cluster
    * Adds the IAM policy from examples/iam-policy.json
    * Adds dynamodb table with history of deployments

## Environment variables

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
