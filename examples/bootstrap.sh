#!/bin/bash
#
# local server
#
# create a .env file with all the variables first,
# then execute this script using ./examples/bootstrap.sh

source .env

make build && \
./ecs-deploy-linux-amd64 --bootstrap \
  --alb-security-groups $ALB_SG \
  --cloudwatch-logs-enabled \
  --cloudwatch-logs-prefix $CLOUDWATCH_LOGS_PREFIX \
  --cluster-name $CLUSTER_NAME \
  --ecs-desired-size 1 \
  --ecs-max-size 1 \
  --ecs-min-size 1 \
  --ecs-security-groups $ECS_SG \
  --ecs-subnets $ECS_SUBNETS \
  --environment $AWS_ACCOUNT_ENV \
  --instance-type t2.micro \
  --key-name $KEY_NAME \
  --loadbalancer-domain cluster.in4it.io \
  --paramstore-enabled \
  --paramstore-kms-arn $PARAMSTORE_KMS_ARN \
  --paramstore-prefix $PARAMSTORE_PREFIX \
  --profile $CLUSTER_AWS_PROFILE \
  --region $CLUSTER_AWS_REGION
