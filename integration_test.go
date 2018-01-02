package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

var region = getEnv("AWS_REGION", "")
var clusterName = getEnv("TEST_CLUSTERNAME", "integrationtest")
var environment = getEnv("TEST_ENVIRONMENT", "")
var albSecurityGroups = getEnv("TEST_ALB_SG", "")
var ecsSubnets = getEnv("TEST_ECS_SUBNETS", "")
var cloudwatchLogsPrefix = getEnv("TEST_CLOUDWATCH_LOGS_PREFIX", "")
var cloudwatchLogsEnabled = getEnv("TEST_CLOUDWATCH_LOGS_ENABLED", "no")
var keyName = getEnv("TEST_KEYNAME", clusterName)
var instanceType = getEnv("TEST_INSTANCETYPE", "t2.micro")
var instanceProfile = getEnv("TEST_INSTANCEPROFILE", clusterName)
var ecsSecurityGroups = getEnv("TEST_ECS_SG", "")
var ecsMinSize = getEnv("TEST_ECS_MINSIZE", "1")
var ecsMaxSize = getEnv("TEST_ECS_MAXSIZE", "1")
var ecsDesiredSize = getEnv("TEST_ECS_DESIREDSIZE", "1")

var ecsDefault = Deploy{
	Cluster:               clusterName,
	ServicePort:           80,
	ServiceProtocol:       "HTTP",
	DesiredCount:          1,
	MinimumHealthyPercent: 100,
	MaximumPercent:        200,
	Containers: []*DeployContainer{
		{
			ContainerName:     "default",
			ContainerTag:      "latest",
			ContainerPort:     80,
			ContainerImage:    "nginx",
			Essential:         true,
			MemoryReservation: 256,
			CPUReservation:    64,
		},
	},
}
var ecsDeploy = Deploy{
	Cluster:               clusterName,
	ServicePort:           8080,
	ServiceProtocol:       "HTTP",
	DesiredCount:          1,
	MinimumHealthyPercent: 100,
	MaximumPercent:        200,
	Containers: []*DeployContainer{
		{
			ContainerName:     "ecs-deploy",
			ContainerTag:      "latest",
			ContainerPort:     8080,
			ContainerImage:    "in4it/ecs-deploy",
			Essential:         true,
			MemoryReservation: 256,
			CPUReservation:    64,
		},
	},
}

func TestClusterIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	// integration test for cluster
	if accountId == nil {
		t.Skip(noAWSMsg)
	}
	teardownFunc := setupTestCluster(t)
	defer teardownFunc(t)
}
func setupTestCluster(t *testing.T) func(t *testing.T) {
	// vars
	ecs := ECS{}
	iam := IAM{}
	service := newService()
	controller := Controller{}
	roleName := "ecs-" + clusterName

	// create dynamodb table
	err := service.createTable()
	if err != nil && !strings.HasPrefix(err.Error(), "ResourceInUseException") {
		t.Errorf("Error: %v\n", err)
	}

	// create instance profile for cluster
	err = iam.getAccountId()
	if err != nil {
		t.Errorf("Error: %v\n", err)
	}
	_, err = iam.createRole(roleName, iam.getEC2IAMTrust())
	if err != nil {
		t.Errorf("Error: %v\n", err)
	}
	var ec2RolePolicy string
	if strings.ToLower(cloudwatchLogsEnabled) == "yes" {
		b, err := ioutil.ReadFile("templates/iam/ecs-ec2-policy-logs.json")
		if err != nil {
			t.Errorf("Error: %v\n", err)
		}
		ec2RolePolicy = strings.Replace(string(b), "${LOGS_RESOURCE}", "arn:aws:logs:"+region+":"+iam.accountId+":log-group:"+cloudwatchLogsPrefix+"-"+environment+":*", -1)
	} else {
		b, err := ioutil.ReadFile("templates/iam/ecs-ec2-policy.json")
		if err != nil {
			t.Errorf("Error: %v\n", err)
		}
		ec2RolePolicy = string(b)
	}
	iam.putRolePolicy(roleName, "ecs-ec2-policy", ec2RolePolicy)

	// wait for role instance profile to exist
	err = iam.createInstanceProfile(roleName)
	if err != nil {
		t.Errorf("Error: %v\n", err)
	}
	err = iam.addRoleToInstanceProfile(roleName, roleName)
	if err != nil {
		t.Errorf("Error: %v\n", err)
	}
	fmt.Println("Waiting until instance profile exists...")
	err = iam.waitUntilInstanceProfileExists(roleName)
	if err != nil {
		t.Errorf("Error: %v\n", err)
	}
	// import key
	b, err := ioutil.ReadFile(getEnv("HOME", "") + "/.ssh/" + keyName)
	if err != nil {
		t.Errorf("Error: %v\n", err)
	}
	pubKey, err := ecs.getPubKeyFromPrivateKey(string(b))
	if err != nil {
		t.Errorf("Error: %v\n", err)
	}
	ecs.importKeyPair(clusterName, pubKey)

	// create launch configuration
	instanceProfile := roleName
	keyName := clusterName
	err = ecs.createLaunchConfiguration(clusterName, keyName, instanceType, instanceProfile, strings.Split(ecsSecurityGroups, ","))
	if err != nil {
		for i := 0; i < 5 && err != nil; i++ {
			if strings.HasPrefix(err.Error(), "RetryableError:") {
				fmt.Println("Error: %v - waiting 10s and retrying...", err.Error())
				time.Sleep(10 * time.Second)
				err = ecs.createLaunchConfiguration(clusterName, keyName, instanceType, instanceProfile, strings.Split(ecsSecurityGroups, ","))
			}
		}
		if err != nil {
			t.Errorf("Fatal Error: %v\n", err)
			// return teardown (couldn't launch)
			return teardown
		}
	}

	// create autoscaling group
	intEcsDesiredSize, _ := strconv.ParseInt(ecsDesiredSize, 10, 64)
	intEcsMaxSize, _ := strconv.ParseInt(ecsMaxSize, 10, 64)
	intEcsMinSize, _ := strconv.ParseInt(ecsMinSize, 10, 64)
	ecs.createAutoScalingGroup(clusterName, intEcsDesiredSize, intEcsMaxSize, intEcsMinSize, strings.Split(ecsSubnets, ","))
	if err != nil {
		t.Errorf("Error: %v\n", err)
	}

	// create cluster
	clusterArn, err := ecs.createCluster(clusterName)
	if err != nil {
		t.Errorf("Error: %v\n", err)
	}
	fmt.Printf("Created ECS Cluster with ARN: %v\n", *clusterArn)
	if albSecurityGroups == "" || ecsSubnets == "" {
		t.Errorf("Incorrect test arguments supplied")
		os.Exit(1)
	}

	// create load balancer, default target, and listener
	alb, err := newALBAndCreate(clusterName, "ipv4", "internet-facing", strings.Split(albSecurityGroups, ","), strings.Split(ecsSubnets, ","), "application")
	if err != nil {
		t.Errorf("Error: %v\n", err)
	}
	defaultTargetGroupArn, err := alb.createTargetGroup("default", ecsDefault)
	if err != nil {
		t.Errorf("Error: %v\n", err)
	}
	err = alb.createListener("HTTP", 80, *defaultTargetGroupArn)
	if err != nil {
		t.Errorf("Error: %v\n", err)
	}

	// deploy
	controller.deploy("ecs-deploy", ecsDeploy)

	// wait until service is stable
	err = ecs.waitUntilServicesStable("ecs-deploy")
	if err != nil {
		t.Errorf("Error: %v\n", err)
	}

	fmt.Println("Waiting before teardown")
	time.Sleep(120 * time.Second)

	// return teardown
	return teardown
}
func teardown(t *testing.T) {
	iam := IAM{}
	ecs := ECS{}
	roleName := "ecs-" + clusterName
	err := ecs.deleteAutoScalingGroup(clusterName, true)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	err = ecs.deleteLaunchConfiguration(clusterName)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	err = ecs.deleteKeyPair(clusterName)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	err = iam.deleteRolePolicy(roleName, "ecs-ec2-policy")
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	err = iam.removeRoleFromInstanceProfile(roleName, roleName)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	err = iam.deleteInstanceProfile(roleName)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	err = iam.deleteRole(roleName)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	alb, err := newALB(clusterName)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	ecsDeployTargetGroup, err := alb.getTargetGroupArn("ecs-deploy")
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	err = alb.deleteTargetGroup(*ecsDeployTargetGroup)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	defaultTargetGroup, err := alb.getTargetGroupArn("default")
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	err = alb.deleteTargetGroup(*defaultTargetGroup)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	for _, v := range alb.listeners {
		err = alb.deleteListener(*v.ListenerArn)
		if err != nil {
			t.Errorf("Error: %v", err)
		}
	}
	err = alb.deleteLoadBalancer()
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	err = ecs.deleteCluster(clusterName)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
}
