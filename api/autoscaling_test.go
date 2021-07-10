package api

import (
	"os"
	"testing"
	"time"

	"github.com/in4it/ecs-deploy/service"
)

type MockService struct {
	GetClusterInfoOutput  *service.DynamoCluster
	IsDeployRunningOutput bool
	service.ServiceIf
}

func (m MockService) GetClusterInfo() (*service.DynamoCluster, error) {
	return m.GetClusterInfoOutput, nil
}

func (m MockService) IsDeployRunning() (bool, error) {
	return m.IsDeployRunningOutput, nil
}

func TestAreAllTasksRunningInCluster(t *testing.T) {
	mc1 := &MockController{
		runningServices: []service.RunningService{
			{
				ServiceName:  "test-service",
				RunningCount: 1,
				PendingCount: 0,
				DesiredCount: 1,
			},
			{
				ServiceName:  "test-service2",
				RunningCount: 2,
				PendingCount: 0,
				DesiredCount: 2,
			},
			{
				ServiceName:  "test-service3",
				RunningCount: 0,
				PendingCount: 0,
				DesiredCount: 0,
			},
		},
	}
	mc2 := &MockController{
		runningServices: []service.RunningService{
			{
				ServiceName:  "test-service",
				RunningCount: 0,
				PendingCount: 0,
				DesiredCount: 1,
			},
		},
	}
	as := AutoscalingController{}

	if !as.areAllTasksRunningInCluster("clustername", mc1) {
		t.Errorf("Expected that all tasks are running in the cluster. Got false")
	}
	if as.areAllTasksRunningInCluster("clustername", mc2) {
		t.Errorf("Expected that all tasks are not running in the cluster. Got true")
	}
}

func TestLaunchProcessPendingScalingOpWithLocking(t *testing.T) {
	// configuration
	os.Setenv("AUTOSCALING_DOWN_PERIOD", "2")
	os.Setenv("AUTOSCALING_DOWN_INTERVAL", "1")
	// mock
	s := MockService{
		IsDeployRunningOutput: false,
		GetClusterInfoOutput: &service.DynamoCluster{
			Identifier: "myService",
			Time:       time.Now(),
			ContainerInstances: []service.DynamoClusterContainerInstance{
				{
					ClusterName:         "testCluster",
					ContainerInstanceId: "1-2-3-4",
					FreeMemory:          int64(2048),
					FreeCpu:             int64(1024),
					Status:              "ACTIVE",
				},
				{
					ClusterName:         "testCluster",
					ContainerInstanceId: "1-2-3-5",
					FreeMemory:          int64(2048),
					FreeCpu:             int64(1024),
					Status:              "ACTIVE",
				},
				{
					ClusterName:         "testCluster",
					ContainerInstanceId: "1-2-3-6",
					FreeMemory:          int64(2048),
					FreeCpu:             int64(1024),
					Status:              "ACTIVE",
				},
			},
		},
	}
	mc1 := &MockController{
		runningServices: []service.RunningService{
			{
				ServiceName:  "myService",
				RunningCount: 1,
				PendingCount: 0,
				DesiredCount: 1,
			},
		},
		getServicesOutput: []*service.DynamoServicesElement{
			{
				C:                 "testCluster",
				S:                 "myService",
				MemoryReservation: int64(2048),
				CpuReservation:    int64(1024),
			},
		},
	}
	// test
	as := AutoscalingController{}
	clusterName := "testCluster"
	pendingScalingOp := "down"
	registeredInstanceCpu := int64(1024)
	registeredInstanceMemory := int64(2048)
	err := as.launchProcessPendingScalingOpWithLocking(clusterName, pendingScalingOp, registeredInstanceCpu, registeredInstanceMemory, s, mc1)
	if err != nil {
		t.Errorf("Error: %s", err)
	}
}
