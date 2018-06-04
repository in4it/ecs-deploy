package test

import (
	"fmt"
	"os"
	"os/signal"
	"testing"
	"time"

	"github.com/in4it/ecs-deploy/api"
	"github.com/in4it/ecs-deploy/provider/ecs"
	"github.com/in4it/ecs-deploy/service"
	"github.com/in4it/ecs-deploy/util"
)

var runIntegrationTest = util.GetEnv("TEST_RUN_INTEGRATION", "no")
var bootstrapFlags = &api.Flags{
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
	LoadBalancers: []service.LoadBalancer{
		{
			Name:          util.GetEnv("TEST_CLUSTERNAME", "integrationtest"),
			IPAddressType: "ipv4",
			Scheme:        "internet-facing",
			Type:          "application",
		},
		{
			Name:          util.GetEnv("TEST_CLUSTERNAME", "integrationtest") + "-2",
			IPAddressType: "ipv4",
			Scheme:        "internet-facing",
			Type:          "application",
		},
	},
}

var ecsDefault = service.Deploy{
	Cluster:               bootstrapFlags.ClusterName,
	ServiceName:           "integrationtest-default",
	ServicePort:           80,
	ServiceProtocol:       "HTTP",
	DesiredCount:          1,
	MinimumHealthyPercent: 100,
	MaximumPercent:        200,
	Containers: []*service.DeployContainer{
		{
			ContainerName:     "integrationtest-default",
			ContainerPort:     80,
			ContainerImage:    "nginx",
			ContainerURI:      "index.docker.io/nginx:alpine",
			Essential:         true,
			MemoryReservation: 128,
			CPUReservation:    64,
			DockerLabels:      map[string]string{"mykey": "myvalue"},
		},
	},
}
var ecsDefaultConcurrentDeploy = service.Deploy{
	Cluster:               bootstrapFlags.ClusterName,
	ServiceName:           "integrationtest-concurrency",
	ServicePort:           80,
	ServiceProtocol:       "HTTP",
	DesiredCount:          1,
	MinimumHealthyPercent: 100,
	MaximumPercent:        200,
	DeregistrationDelay:   5,
	Containers: []*service.DeployContainer{
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
var ecsMultiDeploy = service.DeployServices{
	Services: []service.Deploy{ecsDefault, ecsDefaultConcurrentDeploy},
}
var ecsDefaultWithChanges = service.Deploy{
	Cluster:               bootstrapFlags.ClusterName,
	LoadBalancer:          bootstrapFlags.ClusterName + "-2",
	ServicePort:           80,
	ServiceProtocol:       "HTTP",
	DesiredCount:          1,
	MinimumHealthyPercent: 100,
	MaximumPercent:        200,
	DeregistrationDelay:   0,
	Stickiness: service.DeployStickiness{
		Enabled:  true,
		Duration: 10000,
	},
	Containers: []*service.DeployContainer{
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
var ecsDefaultFailingHealthCheck = service.Deploy{
	Cluster:               bootstrapFlags.ClusterName,
	ServicePort:           80,
	ServiceProtocol:       "HTTP",
	DesiredCount:          1,
	MinimumHealthyPercent: 100,
	MaximumPercent:        200,
	DeregistrationDelay:   5,
	Containers: []*service.DeployContainer{
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
var ecsDeploy = service.Deploy{
	Cluster:               bootstrapFlags.ClusterName,
	ServicePort:           8080,
	ServiceProtocol:       "HTTP",
	DesiredCount:          1,
	MinimumHealthyPercent: 100,
	MaximumPercent:        200,
	DeregistrationDelay:   5,
	Containers: []*service.DeployContainer{
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
	e := ecs.ECS{}
	s := service.NewService()
	controller := api.Controller{}
	clusterName := bootstrapFlags.ClusterName

	// change cur dir
	err = os.Chdir("..")
	if err != nil {
		t.Errorf("Couldn't change directory")
		return shutdown
	}

	err = controller.Bootstrap(bootstrapFlags)
	if err != nil {
		t.Errorf("Couldn't spin up cluster: %v", err.Error())
		return shutdown
	}

	// deploy (3 times: one time to create, one to update and one with different layout)
	var deployRes, deployRes2 *service.DeployResult
	for y := 0; y < 3; y++ {
		s.ServiceName = "integrationtest-default"
		if y == 0 || y == 2 {
			fmt.Println("==> Deploying first ecs service <==")
			deployRes, err = controller.Deploy(s.ServiceName, ecsDefault)
		} else {
			fmt.Println("==> Deploying ecs service with changes <==")
			deployRes, err = controller.Deploy(s.ServiceName, ecsDefaultWithChanges)
		}
		if err != nil {
			t.Errorf("Error: %v\n", err)
			// can't recover from this
			return teardown
		}
		fmt.Printf("Deployed %v with task definition %v\n", deployRes.ServiceName, deployRes.TaskDefinitionArn)

		var deployed bool
		for i := 0; i < 30 && !deployed; i++ {
			dd, err := s.GetDeployment(s.ServiceName, deployRes.DeploymentTime.Format("2006-01-02T15:04:05.999999999Z"))
			if err != nil {
				t.Errorf("Error: %v\n", err)
			}
			if dd != nil && dd.Status == "success" {
				deployed = true
			} else {
				fmt.Printf("Waiting for deploy %v to have status success (latest status: %v)\n", s.ServiceName, dd.Status)
				time.Sleep(30 * time.Second)
			}
		}
		if !deployed {
			fmt.Println("Couldn't deploy service")
			return teardown
		}
	}

	// deploy an update with healthchecks that fail and observe rolling back
	fmt.Println("==> Deploying ecs service that fails <==")
	deployRes2, err = controller.Deploy(s.ServiceName, ecsDefaultFailingHealthCheck)
	var deployed bool
	for i := 0; i < 30 && !deployed; i++ {
		dd, err := s.GetDeployment(s.ServiceName, deployRes2.DeploymentTime.Format("2006-01-02T15:04:05.999999999Z"))
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
	var runningService service.RunningService
	for i := 0; i < 30 && !settled; i++ {
		runningService, err = e.DescribeService(clusterName, s.ServiceName, false, false, false)
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
	controller := api.Controller{}
	err := controller.DeleteCluster(bootstrapFlags)
	if err != nil {
		t.Errorf("Error: %v\n", err)
	}
}
func shutdown(t *testing.T) {
	fmt.Println("Shutting down without teardown")
}
