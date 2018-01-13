package main

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"testing"
	"time"
)

var runIntegrationTest = getEnv("TEST_RUN_INTEGRATION", "no")
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
var paramstoreEnabled = getEnv("TEST_PARAMSTORE_ENABLED", "no")
var randSrc = rand.NewSource(time.Now().UnixNano())

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

var ecsDefault = Deploy{
	Cluster:               clusterName,
	ServicePort:           80,
	ServiceProtocol:       "HTTP",
	DesiredCount:          1,
	MinimumHealthyPercent: 100,
	MaximumPercent:        200,
	Containers: []*DeployContainer{
		{
			ContainerName:     "integrationtest-default",
			ContainerPort:     80,
			ContainerImage:    "nginx",
			ContainerURI:      "index.docker.io/nginx:alpine",
			Essential:         true,
			MemoryReservation: 128,
			CPUReservation:    64,
		},
	},
}
var ecsDefaultWithChanges = Deploy{
	Cluster:               clusterName,
	ServicePort:           80,
	ServiceProtocol:       "HTTP",
	DesiredCount:          1,
	MinimumHealthyPercent: 100,
	MaximumPercent:        200,
	DeregistrationDelay:   0,
	Stickiness: DeployStickiness{
		Enabled:  true,
		Duration: 10000,
	},
	Containers: []*DeployContainer{
		{
			ContainerName:     "integrationtest-default",
			ContainerPort:     80,
			ContainerImage:    "nginx",
			ContainerURI:      "index.docker.io/nginx:alpine",
			Essential:         true,
			MemoryReservation: 128,
			CPUReservation:    64,
		},
	},
}
var ecsDefaultFailingHealthCheck = Deploy{
	Cluster:               clusterName,
	ServicePort:           80,
	ServiceProtocol:       "HTTP",
	DesiredCount:          1,
	MinimumHealthyPercent: 100,
	MaximumPercent:        200,
	Containers: []*DeployContainer{
		{
			ContainerName:     "integrationtest-default",
			ContainerPort:     80,
			ContainerImage:    "nginx",
			ContainerURI:      "index.docker.io/redis:latest",
			Essential:         true,
			MemoryReservation: 128,
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
			ContainerName:     "integrationtest-ecs-deploy",
			ContainerTag:      "latest",
			ContainerPort:     8080,
			ContainerURI:      "index.docker.io/in4it/ecs-deploy:latest",
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
	if runIntegrationTest != "yes" {
		fmt.Println("Skipping integrationtest (env var TEST_RUN_INTEGRATION != yes)")
		t.Skip("skipping integration test")
	}
	// Do you want to run integration test?
	fmt.Println("Going to run integration test in 5s... (You can hit ctrl+c now to abort)")
	time.Sleep(5 * time.Second)
	// setup teardown capture (ctrl+c)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		fmt.Println("Caught SIGINT: running teardown")
		teardown(t)
		os.Exit(1)
	}()
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
	paramstore := Paramstore{}
	service := newService()
	controller := Controller{}
	cloudwatch := CloudWatch{}
	roleName := "ecs-" + clusterName

	// set deploy default
	controller.setDeployDefaults(&ecsDefault)

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
				fmt.Printf("Error: %v - waiting 10s and retrying...\n", err.Error())
				time.Sleep(10 * time.Second)
				err = ecs.createLaunchConfiguration(clusterName, keyName, instanceType, instanceProfile, strings.Split(ecsSecurityGroups, ","))
			}
		}
		if err != nil {
			t.Errorf("Fatal Error: %v\n", err)
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

	// create log group
	err = cloudwatch.createLogGroup(clusterName, cloudwatchLogsPrefix+"-"+environment)
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
	defaultTargetGroupArn, err := alb.createTargetGroup("integrationtest-default", ecsDefault)
	if err != nil {
		t.Errorf("Error: %v\n", err)
	}
	err = alb.createListener("HTTP", 80, *defaultTargetGroupArn)
	if err != nil {
		t.Errorf("Error: %v\n", err)
	}
	// create env vars
	if strings.ToLower(paramstoreEnabled) == "yes" {
		paramstore.putParameter("integrationtest-ecs-deploy", DeployServiceParameter{
			Name:  "JWT_TOKEN",
			Value: RandStringBytesMaskImprSrc(16),
		})
		paramstore.putParameter("integrationtest-ecs-deploy", DeployServiceParameter{
			Name:  "DEPLOY_PASSWORD",
			Value: RandStringBytesMaskImprSrc(8),
		})
		paramstore.putParameter("integrationtest-ecs-deploy", DeployServiceParameter{
			Name:  "URL_PREFIX",
			Value: "/integrationtest-ecs-deploy",
		})
	}

	// wait for autoscaling group to be in service
	fmt.Println("Wait for autoscaling group to be in service")
	ecs.waitForAutoScalingGroupInService(clusterName)
	if err != nil {
		t.Errorf("Error: %v\n", err)
	}

	// deploy (3 times: one time to create, one to update and one with different layout)
	var deployRes *DeployResult
	for y := 0; y < 2; y++ {
		service.serviceName = "integrationtest-default"
		if y == 0 || y == 1 {
			deployRes, err = controller.deploy(service.serviceName, ecsDefault)
		} else {
			deployRes, err = controller.deploy(service.serviceName, ecsDefaultWithChanges)
		}
		if err != nil {
			t.Errorf("Error: %v\n", err)
			// can't recover from this
			return teardown
		}
		fmt.Printf("Deployed %v with task definition %v\n", deployRes.ServiceName, deployRes.TaskDefinitionArn)

		var deployed bool
		for i := 0; i < 30 && !deployed; i++ {
			dd, err := service.getLastDeploy()
			if err != nil {
				t.Errorf("Error: %v\n", err)
			}
			if dd != nil && dd.Status == "success" {
				deployed = true
			} else {
				fmt.Printf("Waiting for deploy %v to have status success (latest status: %v)\n", service.serviceName, dd.Status)
				time.Sleep(30 * time.Second)
			}
		}
		if !deployed {
			fmt.Println("Couldn't deploy service")
			return teardown
		}
	}

	// deploy an update with healthchecks that fail and observe rolling back
	controller.deploy(service.serviceName, ecsDefaultFailingHealthCheck)
	var deployed bool
	for i := 0; i < 30 && !deployed; i++ {
		dd, err := service.getLastDeploy()
		if err != nil {
			t.Errorf("Error: %v\n", err)
		}
		if dd != nil && dd.Status != "running" {
			deployed = true
		} else {
			fmt.Printf("Waiting for deploy to be rolled back (latest status: %v)\n", dd.Status)
			time.Sleep(30 * time.Second)
		}
	}
	settled := false
	var runningService RunningService
	for i := 0; i < 30 && !settled; i++ {
		runningService, err = ecs.describeService(clusterName, service.serviceName, false, false, false)
		if err != nil {
			t.Errorf("Error: %v\n", err)
		}
		if len(runningService.Deployments) == 1 && runningService.Deployments[0].TaskDefinition == deployRes.TaskDefinitionArn {
			settled = true
		} else {
			fmt.Printf("Waiting for deployments to be 1 (currently %d) and task definition to be %v (currently %v)\n",
				len(runningService.Deployments), deployRes.TaskDefinitionArn, runningService.Deployments[0].TaskDefinition)
			time.Sleep(30 * time.Second)
		}
	}
	if !settled {
		t.Errorf("Error: Rollback didn't happen: wrong task definition (expected: %v): %+v\n", deployRes.TaskDefinitionArn, runningService.Deployments)
	} else {
		fmt.Println("Rolled back")
	}

	fmt.Println("Waiting before teardown (or ctrl+c)")
	time.Sleep(120 * time.Second)

	// return teardown
	return teardown
}
func teardown(t *testing.T) {
	iam := IAM{}
	ecs := ECS{}
	paramstore := Paramstore{}
	roleName := "ecs-" + clusterName
	cloudwatch := CloudWatch{}
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
	for _, v := range alb.listeners {
		err = alb.deleteListener(*v.ListenerArn)
		if err != nil {
			t.Errorf("Error: %v", err)
		}
	}
	// will be enabled later
	/*ecsDeployTargetGroup, err := alb.getTargetGroupArn("integrationtest-ecs-deploy")
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	if ecsDeployTargetGroup != nil {
		err = alb.deleteTargetGroup(*ecsDeployTargetGroup)
		if err != nil {
			t.Errorf("Error: %v", err)
		}
	}*/
	defaultTargetGroup, err := alb.getTargetGroupArn("integrationtest-default")
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	if defaultTargetGroup != nil {
		err = alb.deleteTargetGroup(*defaultTargetGroup)
		if err != nil {
			t.Errorf("Error: %v", err)
		}
	}
	err = alb.deleteLoadBalancer()
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	err = ecs.deleteService(clusterName, "integrationtest-default")
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	err = ecs.waitUntilServicesInactive(clusterName, "integrationtest-default")
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	fmt.Println("Wait for autoscaling group to not exist")
	err = ecs.waitForAutoScalingGroupNotExists(clusterName)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	var drained bool
	fmt.Println("Waiting for EC2 instances to drain from ECS cluster")
	for i := 0; i < 5 && !drained; i++ {
		instanceArns, err := ecs.listContainerInstances(clusterName)
		if err != nil {
			t.Errorf("Error: %v", err)
		}
		if len(instanceArns) == 0 {
			drained = true
		} else {
			time.Sleep(5 * time.Second)
		}
	}
	err = ecs.deleteCluster(clusterName)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	err = cloudwatch.deleteLogGroup(cloudwatchLogsPrefix + "-" + environment)
	if err != nil {
		t.Errorf("Error: %v\n", err)
	}
	paramstore.deleteParameter("integrationtest-ecs-deploy", "JWT_TOKEN")
	if err != nil {
		t.Errorf("Error: %v\n", err)
	}
	paramstore.deleteParameter("integrationtest-ecs-deploy", "DEPLOY_PASSWORD")
	if err != nil {
		t.Errorf("Error: %v\n", err)
	}
	paramstore.deleteParameter("integrationtest-ecs-deploy", "URL_PREFIX")
	if err != nil {
		t.Errorf("Error: %v\n", err)
	}
}

// stackoverflow
func RandStringBytesMaskImprSrc(n int) string {
	b := make([]byte, n)
	// A randSrc.Int63() generates 63 random bits, enough for letterIdxMax characters!
	for i, cache, remain := n-1, randSrc.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = randSrc.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return string(b)
}
