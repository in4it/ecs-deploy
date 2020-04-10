#!/bin/bash
echo 'ECS_CLUSTER=${CLUSTER_NAME}' > /etc/ecs/ecs.config
start ecs
%{ if YUM_PROXY_URL != "" }echo 'proxy=${YUM_PROXY_URL}' >> /etc/yum.conf%{ endif ~}
