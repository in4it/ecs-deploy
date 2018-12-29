#!/bin/bash
echo 'ECS_CLUSTER=${CLUSTER_NAME}' > /etc/ecs/ecs.config
start ecs
