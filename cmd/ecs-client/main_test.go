package main

import (
	"testing"
)

func TestGetDeployDataWithService(t *testing.T) {
	var resultJson = `{"services":[{"cluster":"mycluster","loadBalancer":"","serviceName":"testservice-worker","servicePort":0,"serviceProtocol":"none","desiredCount":1,"minimumHealthyPercent":0,"maximumPercent":0,"containers":[{"containerName":"testservice-worker","containerTag":"","containerPort":0,"containerCommand":null,"containerImage":"echoserver","containerURI":"gcr.io/google_containers/echoserver:1.4","containerEntryPoint":null,"essential":true,"memory":0,"memoryReservation":64,"cpu":0,"cpuReservation":0,"dockerLabels":null,"healthCheck":{"command":null,"interval":0,"timeout":0,"retries":0,"startPeriod":0},"environment":null,"mountPoints":null,"ulimits":null,"links":null,"logConfiguration":{"logDriver":"","options":{"max-size":"","max-file":""}},"portMappings":null}],"healthCheck":{"healthyThreshold":0,"unhealthyThreshold":0,"path":"","port":"","protocol":"","interval":0,"matcher":"","timeout":0,"gracePeriodSeconds":0},"ruleConditions":null,"networkMode":"","networkConfiguration":{"assignPublicIp":"","securityGroups":null,"subnets":null},"placementConstraints":null,"launchType":"","deregistrationDelay":-1,"stickiness":{"enabled":false,"duration":-1},"volumes":null,"envNamespace":"","serviceRegistry":"","schedulingStrategy":"","appMesh":""},{"cluster":"mycluster","loadBalancer":"","serviceName":"testservice","servicePort":80,"serviceProtocol":"HTTP","desiredCount":1,"minimumHealthyPercent":0,"maximumPercent":0,"containers":[{"containerName":"testservice","containerTag":"","containerPort":80,"containerCommand":null,"containerImage":"nginx","containerURI":"index.docker.io/nginx:alpine","containerEntryPoint":null,"essential":true,"memory":0,"memoryReservation":128,"cpu":0,"cpuReservation":0,"dockerLabels":null,"healthCheck":{"command":null,"interval":0,"timeout":0,"retries":0,"startPeriod":0},"environment":null,"mountPoints":null,"ulimits":null,"links":null,"logConfiguration":{"logDriver":"","options":{"max-size":"","max-file":""}},"portMappings":null}],"healthCheck":{"healthyThreshold":3,"unhealthyThreshold":3,"path":"/","port":"","protocol":"","interval":60,"matcher":"200,301","timeout":0,"gracePeriodSeconds":0},"ruleConditions":[{"listeners":["http","https"],"pathPattern":"","hostname":"testservice","cognitoAuth":{"userPoolName":"","clientName":""}}],"networkMode":"","networkConfiguration":{"assignPublicIp":"","securityGroups":null,"subnets":null},"placementConstraints":null,"launchType":"","deregistrationDelay":30,"stickiness":{"enabled":false,"duration":-1},"volumes":null,"envNamespace":"","serviceRegistry":"","schedulingStrategy":"","appMesh":""}]}`

	deployServices, err := getDeployDataWithService("testservice", "testdata")
	if err != nil {
		t.Errorf("error: %v\n", err)
	}
	deployData, err := convertDeployServiceToJson(deployServices)
	if err != nil {
		t.Errorf("error: %v\n", err)
	}

	if resultJson[0:150] != deployData[0:150] {
		t.Errorf("JSON doesn't match:\nGot: %v\nExpected: %v", deployData, resultJson)
	}
}
