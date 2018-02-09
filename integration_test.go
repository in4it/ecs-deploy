package ecsdeploy

import (
	"fmt"
	"os"
	"os/signal"
	"testing"
	"time"

	"github.com/in4it/ecs-deploy/util"
)

var runIntegrationTest = util.GetEnv("TEST_RUN_INTEGRATION", "no")
var bootstrapFlags = &Flags{
	Region:                util.GetEnv("AWS_REGION", ""),
	ClusterName:           util.GetEnv("TEST_CLUSTERNAME", "integrationtest"),
	Environment:           util.GetEnv("TEST_ENVIRONMENT", ""),
	AlbSecurityGroups:     util.GetEnv("TEST_ALB_SG", ""),
	EcsSubnets:            util.GetEnv("TEST_ECS_SUBNETS", ""),
	CloudwatchLogsPrefix:  util.GetEnv("TEST_CLOUDWATCH_LOGS_PREFIX", ""),
	CloudwatchLogsEnabled: util.YesNoToBool(util.GetEnv("TEST_CLOUDWATCH_LOGS_ENABLED", "no")),
	KeyName:               util.GetEnv("TEST_KEYNAME", util.GetEnv("TEST_CLUSTERNAME", "integrationtest")),
	InstanceType:          util.GetEnv("TEST_INSTANCETYPE", "t2.micro"),
	EcsSecurityGroups:     util.GetEnv("TEST_ECS_SG", ""),
	EcsMinSize:            util.GetEnv("TEST_ECS_MINSIZE", "1"),
	EcsMaxSize:            util.GetEnv("TEST_ECS_MAXSIZE", "1"),
	EcsDesiredSize:        util.GetEnv("TEST_ECS_DESIREDSIZE", "1"),
	ParamstoreEnabled:     util.YesNoToBool(util.GetEnv("TEST_PARAMSTORE_ENABLED", "no")),
	DisableEcsDeploy:      true,
}

var ecsDefault = Deploy{
	Cluster:               bootstrapFlags.ClusterName,
	ServiceName:           "integrationtest-default",
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
var ecsDefaultConcurrentDeploy = Deploy{
	Cluster:               bootstrapFlags.ClusterName,
	ServiceName:           "integrationtest-concurrency",
	ServicePort:           80,
	ServiceProtocol:       "HTTP",
	DesiredCount:          1,
	MinimumHealthyPercent: 100,
	MaximumPercent:        200,
	DeregistrationDelay:   5,
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
var ecsMultiDeploy = DeployServices{
	Services: []Deploy{ecsDefault, ecsDefaultConcurrentDeploy},
}
var ecsDefaultWithChanges = Deploy{
	Cluster:               bootstrapFlags.ClusterName,
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
	Cluster:               bootstrapFlags.ClusterName,
	ServicePort:           80,
	ServiceProtocol:       "HTTP",
	DesiredCount:          1,
	MinimumHealthyPercent: 100,
	MaximumPercent:        200,
	DeregistrationDelay:   5,
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
	Cluster:               bootstrapFlags.ClusterName,
	ServicePort:           8080,
	ServiceProtocol:       "HTTP",
	DesiredCount:          1,
	MinimumHealthyPercent: 100,
	MaximumPercent:        200,
	DeregistrationDelay:   5,
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
	var err error
	ecs := ECS{}
	service := newService()
	controller := Controller{}
	clusterName := bootstrapFlags.ClusterName

	err = controller.Bootstrap(bootstrapFlags)
	if err != nil {
		t.Errorf("Couldn't spin up cluster: %v", err.Error())
		return shutdown
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
	controller := Controller{}
	err := controller.DeleteCluster(bootstrapFlags)
	if err != nil {
		t.Errorf("Error: %v\n", err)
	}
}
func shutdown(t *testing.T) {
	fmt.Println("Shutting down without teardown")
}
