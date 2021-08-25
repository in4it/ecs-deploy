package api

import (
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/in4it/ecs-deploy/provider/ecs"
	"github.com/in4it/ecs-deploy/service"
	"github.com/juju/loggo"
)

type MockECS struct {
	ecs.ECSIf
	GetInstanceResourcesOutputFree       []ecs.FreeInstanceResource
	GetInstanceResourcesOutputRegistered []ecs.RegisteredInstanceResource
}

type MockService struct {
	GetClusterInfoOutput  *service.DynamoCluster
	GetClusterInfoCounter uint64
	IsDeployRunningOutput bool
	PutClusterInfoOutput  *service.DynamoCluster
	PutClusterInfoCounter uint64
	service.ServiceIf
}

type MockAutoScaling struct {
	ecs.AutoScalingIf
	GetAutoScalingGroupByTagOutput string
}

func (m *MockECS) GetInstanceResources(clusterName string) ([]ecs.FreeInstanceResource, []ecs.RegisteredInstanceResource, error) {
	return m.GetInstanceResourcesOutputFree, m.GetInstanceResourcesOutputRegistered, nil
}

func (m *MockAutoScaling) GetAutoScalingGroupByTag(clusterName string) (string, error) {
	return m.GetAutoScalingGroupByTagOutput, nil
}

func (m *MockAutoScaling) ScaleClusterNodes(autoScalingGroupName string, change int64) error {
	return nil

}
func (m *MockService) PutClusterInfo(dc service.DynamoCluster, clusterName string, action string, pendingAction string) (*service.DynamoCluster, error) {
	atomic.AddUint64(&m.PutClusterInfoCounter, 1)
	m.GetClusterInfoOutput.ScalingOperation.PendingAction = pendingAction
	return m.PutClusterInfoOutput, nil
}
func (m *MockService) GetClusterInfo() (*service.DynamoCluster, error) {
	atomic.AddUint64(&m.GetClusterInfoCounter, 1)
	return m.GetClusterInfoOutput, nil
}

func (m *MockService) IsDeployRunning() (bool, error) {
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
	asAutoscalingControllerLogger.SetLogLevel(loggo.DEBUG)
	// mock
	am := &MockAutoScaling{
		GetAutoScalingGroupByTagOutput: "ecs-deploy",
	}
	s := &MockService{
		IsDeployRunningOutput: false,
		GetClusterInfoOutput: &service.DynamoCluster{
			Identifier: "myService",
			Time:       time.Now(),
			ScalingOperation: service.DynamoClusterScalingOperation{
				ClusterName:   "testCluster",
				Action:        "down",
				PendingAction: "down",
			},
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
		PutClusterInfoOutput: &service.DynamoCluster{
			Identifier: "myService",
			Time:       time.Now(),
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
	var (
		err1 error
		err2 error
	)

	wait1 := make(chan struct{})
	wait2 := make(chan struct{})

	go func() {
		err1 = as.launchProcessPendingScalingOpWithLocking(clusterName, pendingScalingOp, registeredInstanceCpu, registeredInstanceMemory, s, mc1, am)
		if err1 != nil {
			t.Errorf("Error: %s", err1)
		}
		close(wait1)

	}()
	go func() {
		err2 = as.launchProcessPendingScalingOpWithLocking(clusterName, pendingScalingOp, registeredInstanceCpu, registeredInstanceMemory, s, mc1, am)
		if err2 != nil {
			t.Errorf("Error: %s", err2)
		}
		close(wait2)

	}()
	<-wait1
	<-wait2

	if s.PutClusterInfoCounter != 1 {
		t.Errorf("PutClusterInfoCounter is %d (expected 1)", s.PutClusterInfoCounter)
	}
	if s.GetClusterInfoCounter != 3 {
		t.Errorf("GetClusterInfoCounter is %d (expected 3)", s.GetClusterInfoCounter)
	}
}

func TestGetClusterInfoWithExpiredCache(t *testing.T) {
	scalingOp := service.DynamoClusterScalingOperation{
		ClusterName:   "testCluster",
		Action:        "down",
		PendingAction: "down",
	}
	e := &MockECS{
		GetInstanceResourcesOutputFree: []ecs.FreeInstanceResource{
			{
				InstanceId:       "i-123",
				AvailabilityZone: "eu-west-1a",
				Status:           "ACTIVE",
			},
			{
				InstanceId:       "i-456",
				AvailabilityZone: "eu-west-1b",
				Status:           "ACTIVE",
			},
		},
		GetInstanceResourcesOutputRegistered: []ecs.RegisteredInstanceResource{},
	}
	s := &MockService{
		IsDeployRunningOutput: false,
		GetClusterInfoOutput: &service.DynamoCluster{
			Identifier:       "myService",
			Time:             time.Now().Truncate(10 * time.Minute),
			ScalingOperation: scalingOp,
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
	as := AutoscalingController{}

	res, err := as.getClusterInfo("myCluster", true, s, e)

	if err != nil {
		t.Errorf("Error getClusterInfo: %s", err)
	}
	if res.ScalingOperation.PendingAction != scalingOp.PendingAction {
		t.Errorf("Scaling Operation not found in result: expected %s, got %s", scalingOp.PendingAction, res.ScalingOperation.PendingAction)
	}
	if len(res.ContainerInstances) != 2 {
		t.Errorf("wrong number of container instances, got: %d", len(res.ContainerInstances))
	}
	if res.ContainerInstances[0].ContainerInstanceId != "i-123" {
		t.Errorf("wrong container instance returned")
	}
}
