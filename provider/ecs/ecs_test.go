package ecs

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/ghodss/yaml"

	"github.com/in4it/ecs-deploy/service"
	"github.com/in4it/ecs-deploy/util"
)

func initDeployment() (service.DeployServices, error) {
	var (
		d service.DeployServices
	)
	dat, err := ioutil.ReadFile("testdata/ecs.yaml")
	if err != nil {
		return d, err
	}

	err = yaml.Unmarshal(dat, &d)
	if err != nil {
		return d, err
	}

	if len(d.Services) == 0 {
		return d, fmt.Errorf("No services found in yaml")
	}

	return d, nil
}

func TestGetNetworkConfiguration(t *testing.T) {
	var (
		err error
	)

	d, err := initDeployment()
	if err != nil {
		t.Errorf("initDeployment failed: %s", err)
		return
	}

	ecs := ECS{}
	networkConfiguration := ecs.getNetworkConfiguration(d.Services[0])

	if *networkConfiguration.AwsvpcConfiguration.AssignPublicIp != "DISABLED" {
		t.Errorf("Incorrect value for assign public ip: %s", *networkConfiguration.AwsvpcConfiguration.AssignPublicIp)
		return
	}

	if len(networkConfiguration.AwsvpcConfiguration.SecurityGroups) == 0 {
		t.Errorf("No security groups found")
		return
	}

	if *networkConfiguration.AwsvpcConfiguration.SecurityGroups[0] != "sg-0123456abc" {
		t.Errorf("Wrong security group")
		return
	}

	if len(networkConfiguration.AwsvpcConfiguration.Subnets) == 0 {
		t.Errorf("No subnets found")
		return
	}

	if *networkConfiguration.AwsvpcConfiguration.Subnets[0] != "subnet-0123456abc" {
		t.Errorf("Wrong security group")
		return
	}
}
func TestCreateTaskDefinition(t *testing.T) {
	var (
		secrets map[string]string
		err     error
	)

	d, err := initDeployment()
	if err != nil {
		t.Errorf("initDeployment failed: %s", err)
		return
	}

	ecs := ECS{}
	err = ecs.CreateTaskDefinitionInput(d.Services[0], secrets, "0123456789")
	if err != nil {
		t.Errorf("Error: %s", err)
	}

	// checks
	if len(ecs.TaskDefinition.ContainerDefinitions) == 0 {
		t.Errorf("No container definition found")
	}

	if *ecs.TaskDefinition.ContainerDefinitions[0].Name != "demo" {
		t.Errorf("Incorrect container definition: name is not demo (got: %s)", *ecs.TaskDefinition.ContainerDefinitions[0].Name)
		return
	}
	if ecs.TaskDefinition.ContainerDefinitions[0].LogConfiguration == nil {
		t.Errorf("Incorrect container definition: no logdriver configured")
		return
	}
	if *ecs.TaskDefinition.ContainerDefinitions[0].LogConfiguration.LogDriver != "json-file" {
		t.Errorf("Incorrect container definition: incorrect log driver (got: %s)", *ecs.TaskDefinition.ContainerDefinitions[0].LogConfiguration.LogDriver)
		return
	}
	if val := ecs.TaskDefinition.ContainerDefinitions[0].LogConfiguration.Options["max-size"]; val == nil {
		t.Errorf("Incorrect container definition: missign max-size log option")
		return
	}
	if *ecs.TaskDefinition.ContainerDefinitions[0].LogConfiguration.Options["max-size"] != "20m" {
		t.Errorf("Incorrect container definition: incorrect log option (got: %s)", *ecs.TaskDefinition.ContainerDefinitions[0].LogConfiguration.Options["max-size"])
		return
	}
	if *ecs.TaskDefinition.ContainerDefinitions[0].LogConfiguration.Options["max-file"] != "1" {
		t.Errorf("Incorrect container definition: incorrect log option (got: %s)", *ecs.TaskDefinition.ContainerDefinitions[0].LogConfiguration.Options["max-file"])
		return
	}

}

func TestWaitUntilServicesStable(t *testing.T) {
	if accountId == nil {
		t.Skip(noAWSMsg)
	}
	ecs := ECS{}
	err := ecs.WaitUntilServicesStable(util.GetEnv("TEST_CLUSTERNAME", "test-cluster"), util.GetEnv("TEST_SERVICENAME", "ecs-deploy"), 10)
	if err != nil {
		t.Errorf("Error: %v", err)
		return
	}
}
func TestEcsTaskMetadata(t *testing.T) {
	input := `{
  "Cluster": "default",
  "TaskARN": "arn:aws:ecs:us-west-2:012345678910:task/9781c248-0edd-4cdb-9a93-f63cb662a5d3",
  "Family": "nginx",
  "Revision": "5",
  "DesiredStatus": "RUNNING",
  "KnownStatus": "RUNNING",
  "Containers": [
    {
      "DockerId": "731a0d6a3b4210e2448339bc7015aaa79bfe4fa256384f4102db86ef94cbbc4c",
      "Name": "~internal~ecs~pause",
      "DockerName": "ecs-nginx-5-internalecspause-acc699c0cbf2d6d11700",
      "Image": "amazon/amazon-ecs-pause:0.1.0",
      "ImageID": "",
      "Labels": {
        "com.amazonaws.ecs.cluster": "default",
        "com.amazonaws.ecs.container-name": "~internal~ecs~pause",
        "com.amazonaws.ecs.task-arn": "arn:aws:ecs:us-west-2:012345678910:task/9781c248-0edd-4cdb-9a93-f63cb662a5d3",
        "com.amazonaws.ecs.task-definition-family": "nginx",
        "com.amazonaws.ecs.task-definition-version": "5"
      },
      "DesiredStatus": "RESOURCES_PROVISIONED",
      "KnownStatus": "RESOURCES_PROVISIONED",
      "Limits": {
        "CPU": 0,
        "Memory": 0
      },
      "CreatedAt": "2018-02-01T20:55:08.366329616Z",
      "StartedAt": "2018-02-01T20:55:09.058354915Z",
      "Type": "CNI_PAUSE",
      "Networks": [
        {
          "NetworkMode": "awsvpc",
          "IPv4Addresses": [
            "10.0.2.106"
          ]
        }
      ]
    },
    {
      "DockerId": "43481a6ce4842eec8fe72fc28500c6b52edcc0917f105b83379f88cac1ff3946",
      "Name": "nginx-curl",
      "DockerName": "ecs-nginx-5-nginx-curl-ccccb9f49db0dfe0d901",
      "Image": "nrdlngr/nginx-curl",
      "ImageID": "sha256:2e00ae64383cfc865ba0a2ba37f61b50a120d2d9378559dcd458dc0de47bc165",
      "Labels": {
        "com.amazonaws.ecs.cluster": "default",
        "com.amazonaws.ecs.container-name": "nginx-curl",
        "com.amazonaws.ecs.task-arn": "arn:aws:ecs:us-west-2:012345678910:task/9781c248-0edd-4cdb-9a93-f63cb662a5d3",
        "com.amazonaws.ecs.task-definition-family": "nginx",
        "com.amazonaws.ecs.task-definition-version": "5"
      },
      "DesiredStatus": "RUNNING",
      "KnownStatus": "RUNNING",
      "Limits": {
        "CPU": 512,
        "Memory": 512
      },
      "CreatedAt": "2018-02-01T20:55:10.554941919Z",
      "StartedAt": "2018-02-01T20:55:11.064236631Z",
      "Type": "NORMAL",
      "Networks": [
        {
          "NetworkMode": "awsvpc",
          "IPv4Addresses": [
            "10.0.2.106"
          ]
        }
      ]
    }
  ],
  "PullStartedAt": "2018-02-01T20:55:09.372495529Z",
  "PullStoppedAt": "2018-02-01T20:55:10.552018345Z"
}`
	var task EcsTaskMetadata
	err := json.Unmarshal([]byte(input), &task)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	split := strings.Split(task.TaskARN, "task/")
	if len(split) != 2 {
		t.Errorf("Error: %v", err)
	}
	if len(split[1]) == 0 {
		t.Errorf("Error: %v", err)
	}
}
