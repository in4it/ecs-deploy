#!/bin/bash

#
# example of accessing API
#
# note: configure .env first
#

#
# get env vars
export $(egrep -v '^#' .env | xargs)

# auth
TOKEN=`curl -X POST localhost:8080/login -H 'Content-type: application/json' -s -d '{"username":"deploy","password":"'${DEPLOY_PASSWORD}'"}' |jq -r '.token'`

if [ "$1" == "createrepo" ] ; then
  curl -X POST localhost:8080/api/v1/ecr/create/myservice -H "Authorization:Bearer ${TOKEN}"
elif [ "$1" == "export" ] ; then
  curl -X GET localhost:8080/api/v1/export/terraform -H "Authorization:Bearer ${TOKEN}"
elif [ "$1" == "deploy" ] ; then
  curl -X POST localhost:8080/api/v1/deploy/myservice -H 'Content-type: application/json' -H "Authorization:Bearer ${TOKEN}" -d \
'{
  "cluster": "ward",
  "servicePort": 3000,
  "serviceProtocol": "HTTP",
  "desiredCount": 1,
  "containers": [
    {
      "containerName": "myservice",
      "containerTag": "latest",
      "containerPort": 3000,
      "memoryReservation": 512,
      "essential": true
    }
  ],
  "healthCheck": {
    "healthyThreshold": 3,
    "unhealthyThreshold": 3,
    "path": "/",
    "interval": 60,
    "matcher": "200,301"
  }
}'
else
  echo 'Usage: curl-api.sh <createrepo|export|deploy>'
fi
