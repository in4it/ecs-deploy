package ecs

import (
	"encoding/json"
	"fmt"
	"os"
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
	dat, err := os.ReadFile("testdata/ecs.yaml")
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

func TestGetMaxWaitMinutes(t *testing.T) {
	ecs := ECS{}
	if num := ecs.getMaxWaitMinutes(0); num != 15 {
		t.Errorf("Got wrong maxWaitMinutes: %d", num)
	}
	if num := ecs.getMaxWaitMinutes(100); num != 20 {
		t.Errorf("Got wrong maxWaitMinutes: %d", num)
	}
	os.Setenv("DEPLOY_MAX_WAIT_SECONDS", "1800")
	if num := ecs.getMaxWaitMinutes(0); num != 30 {
		t.Errorf("Got wrong maxWaitMinutes: %d", num)
	}
	if num := ecs.getMaxWaitMinutes(300); num != 30 {
		t.Errorf("Got wrong maxWaitMinutes: %d", num)
	}
}

func TestGetPubKeyFromPrivateKey(t *testing.T) {
	testPrivateKey := `-----BEGIN OPENSSH PRIVATE KEY-----
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
AQIDBA==
-----END OPENSSH PRIVATE KEY-----`
	e := ECS{}
	_, err := e.GetPubKeyFromPrivateKey(testPrivateKey)
	if err != nil {
		t.Errorf("cannot parse private key: %s", err)
	}
}
