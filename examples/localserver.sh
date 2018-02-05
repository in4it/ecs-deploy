#!/bin/bash
#
# local server
#
# create a .env file with all the variables first,
# then execute this script using ./examples/localserver.sh

make build-server && \

eval $(egrep -v '^#' .env | xargs) ./ecs-deploy-linux-amd64 --server 
