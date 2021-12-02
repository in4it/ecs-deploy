package api

import (
	"github.com/aws/aws-sdk-go/aws"
	"os"
	"sync/atomic"
	"testing"
	"time"

	ecsService "github.com/aws/aws-sdk-go/service/ecs"
	"github.com/in4it/ecs-deploy/provider/ecs"
	"github.com/in4it/ecs-deploy/service"
	"github.com/juju/loggo"
)

type MockECS struct {
	ecs.ECSIf
	GetInstanceResourcesOutputFree       []ecs.FreeInstanceResource
	GetInstanceResourcesOutputRegistered []ecs.RegisteredInstanceResource
	ConvertResourceToRirOutput           ecs.RegisteredInstanceResource
	ConvertResourceToFirOutput           ecs.FreeInstanceResource
	DescribeServicesWithOptionsOutput    []service.RunningService
}

type MockService struct {
	GetClusterInfoOutput  *service.DynamoCluster
	GetClusterInfoCounter uint64
	IsDeployRunningOutput bool
	PutClusterInfoOutput  *service.DynamoCluster
	PutClusterInfoCounter uint64
	GetServicesOutput     *service.DynamoServices
	service.ServiceIf
}

type MockAutoScaling struct {
	ecs.AutoScalingIf
	GetAutoScalingGroupByTagOutput  string
	GetAutoScalingGroupByTagsOutput string
}

func (m *MockECS) GetInstanceResources(clusterName string) ([]ecs.FreeInstanceResource, []ecs.RegisteredInstanceResource, error) {
	return m.GetInstanceResourcesOutputFree, m.GetInstanceResourcesOutputRegistered, nil
}
func (m *MockECS) ConvertResourceToFir(cir []ecs.ContainerInstanceResource) (ecs.FreeInstanceResource, error) {
	return m.ConvertResourceToFirOutput, nil
}
func (m *MockECS) ConvertResourceToRir(cir []ecs.ContainerInstanceResource) (ecs.RegisteredInstanceResource, error) {
	return m.ConvertResourceToRirOutput, nil
}
func (m *MockECS) DescribeServicesWithOptions(clusterName string, serviceNames []*string, showEvents bool, showTasks bool, showStoppedTasks bool, options map[string]string) ([]service.RunningService, error) {
	return m.DescribeServicesWithOptionsOutput, nil
}

func (m *MockAutoScaling) GetAutoScalingGroupByTag(clusterName string) (string, error) {
	return m.GetAutoScalingGroupByTagOutput, nil
}
func (m *MockAutoScaling) GetAutoScalingGroupByTags(name string, arch string) (string, error) {
	return m.GetAutoScalingGroupByTagsOutput, nil
}

func (m *MockAutoScaling) ScaleClusterNodes(autoScalingGroupName string, change int64) error {
	return nil
}
func (m *MockAutoScaling) GetClusterNodeDesiredCount(autoScalingGroupName string) (int64, int64, int64, error) {
	return 1, 1, 5, nil
}

func (m *MockService) PutClusterInfo(dc service.DynamoCluster, clusterName string, action string, pendingAction string, arch string) (*service.DynamoCluster, error) {
	atomic.AddUint64(&m.PutClusterInfoCounter, 1)
	m.GetClusterInfoOutput.ScalingOperation.PendingAction = pendingAction
	m.GetClusterInfoOutput.ScalingOperation.Action = action
	m.GetClusterInfoOutput.ScalingOperation.CPUArchitecture = arch
	return m.PutClusterInfoOutput, nil
}
func (m *MockService) GetClusterInfo() (*service.DynamoCluster, error) {
	atomic.AddUint64(&m.GetClusterInfoCounter, 1)
	return m.GetClusterInfoOutput, nil
}
func (m *MockService) IsDeployRunning() (bool, error) {
	return m.IsDeployRunningOutput, nil
}
func (m *MockService) GetScalingActivity(clusterName string, startTime time.Time) (string, string, error) {
	return "no", "", nil
}

func (m *MockService) AutoscalingPullInit() error {
	return nil
}
func (m *MockService) AutoscalingPullAcquireLock(localId string) (bool, error) {
	return true, nil
}
func (m *MockService) GetServices(ds *service.DynamoServices) error {
	ds.ServiceName = "__SERVICES"
	ds.Services = m.GetServicesOutput.Services
	return nil
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
		GetAutoScalingGroupByTagOutput:  "ecs-deploy",
		GetAutoScalingGroupByTagsOutput: "ecs-deploy",
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
		err1 = as.launchProcessPendingScalingOpWithLocking(clusterName, pendingScalingOp, registeredInstanceCpu, registeredInstanceMemory, s, mc1, am, "x86_64")
		if err1 != nil {
			t.Errorf("Error: %s", err1)
		}
		close(wait1)

	}()
	go func() {
		err2 = as.launchProcessPendingScalingOpWithLocking(clusterName, pendingScalingOp, registeredInstanceCpu, registeredInstanceMemory, s, mc1, am, "x86_64")
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
		ClusterName:     "testCluster",
		Action:          "down",
		PendingAction:   "down",
		CPUArchitecture: "x86_64",
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
					CPUArchitecture:     "x86_64",
				},
				{
					ClusterName:         "testCluster",
					ContainerInstanceId: "1-2-3-5",
					FreeMemory:          int64(2048),
					FreeCpu:             int64(1024),
					Status:              "ACTIVE",
					CPUArchitecture:     "x86_64",
				},
				{
					ClusterName:         "testCluster",
					ContainerInstanceId: "1-2-3-6",
					FreeMemory:          int64(2048),
					FreeCpu:             int64(1024),
					Status:              "ACTIVE",
					CPUArchitecture:     "x86_64",
				},
				{
					ClusterName:         "testCluster",
					ContainerInstanceId: "1-2-3-7",
					FreeMemory:          int64(2048),
					FreeCpu:             int64(1024),
					Status:              "ACTIVE",
					CPUArchitecture:     "arm64",
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

func TestGetClusterInfoWithExpiredCacheARM(t *testing.T) {
	scalingOp := service.DynamoClusterScalingOperation{
		ClusterName:     "testCluster",
		Action:          "down",
		PendingAction:   "down",
		CPUArchitecture: "arm64",
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
					CPUArchitecture:     "x86_64",
				},
				{
					ClusterName:         "testCluster",
					ContainerInstanceId: "1-2-3-5",
					FreeMemory:          int64(2048),
					FreeCpu:             int64(1024),
					Status:              "ACTIVE",
					CPUArchitecture:     "x86_64",
				},
				{
					ClusterName:         "testCluster",
					ContainerInstanceId: "1-2-3-6",
					FreeMemory:          int64(2048),
					FreeCpu:             int64(1024),
					Status:              "ACTIVE",
					CPUArchitecture:     "x86_64",
				},
				{
					ClusterName:         "testCluster",
					ContainerInstanceId: "1-2-3-7",
					FreeMemory:          int64(2048),
					FreeCpu:             int64(1024),
					Status:              "ACTIVE",
					CPUArchitecture:     "arm64",
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

func TestProcessEcsMessage(t *testing.T) {
	asAutoscalingControllerLogger.SetLogLevel(loggo.DEBUG)
	message := ecs.SNSPayloadEcs{
		Detail: ecs.SNSPayloadEcsDetail{
			ClusterArn: "arn:aws:ecs:us-west-2:123456789012:cluster/testCluster",
			Attributes: []ecs.SNSPayloadEcsDetailAttributes{
				{
					Name:  "ecs.cpu-architecture",
					Value: "x86_64",
				},
				{
					Name:  "ecs.availability-zone",
					Value: "us-east-1a",
				},
			},
		},
	}
	mc := &MockController{
		runningServices: []service.RunningService{
			{
				ServiceName:  "test-service1",
				RunningCount: 1,
				PendingCount: 0,
				DesiredCount: 1,
			},
			{
				ServiceName:  "test-service2",
				RunningCount: 3,
				PendingCount: 0,
				DesiredCount: 3,
			},
		},
		getServicesOutput: []*service.DynamoServicesElement{
			{
				C:                 "testCluster",
				S:                 "test-service1",
				MemoryReservation: int64(2048),
				CpuReservation:    int64(1024),
			},
			{
				C:                 "testCluster",
				S:                 "test-service2",
				MemoryReservation: int64(4096),
				CpuReservation:    int64(1024),
			},
		},
	}
	e := &MockECS{
		ConvertResourceToRirOutput: ecs.RegisteredInstanceResource{
			InstanceId:       "i-test",
			RegisteredMemory: 16384,
			RegisteredCpu:    4096,
		},
		ConvertResourceToFirOutput: ecs.FreeInstanceResource{
			InstanceId:       "i-test",
			AvailabilityZone: "us-east-1a",
			Status:           "ACTIVE",
			FreeMemory:       2048,
			FreeCpu:          1024,
		},
		GetInstanceResourcesOutputFree: []ecs.FreeInstanceResource{
			{
				InstanceId:       "i-test",
				AvailabilityZone: "eu-east-1a",
				Status:           "ACTIVE",
				FreeMemory:       6144,
				FreeCpu:          1024,
			},
		},
	}
	s := &MockService{}

	mockAutoscaling := &MockAutoScaling{
		GetAutoScalingGroupByTagOutput:  "autoscalingGroup",
		GetAutoScalingGroupByTagsOutput: "autoscalingGroup",
	}

	as := AutoscalingController{}
	err := as.processEcsMessage(message, mc, e, s, mockAutoscaling)
	if err != nil {
		t.Errorf("processEcsMessage error: %s", err)
	}
}

func TestStartAutoscalingPollingStrategy(t *testing.T) {
	asAutoscalingControllerLogger.SetLogLevel(loggo.DEBUG)
	e := &MockECS{
		ConvertResourceToRirOutput: ecs.RegisteredInstanceResource{
			InstanceId:       "i-test",
			RegisteredMemory: 16384,
			RegisteredCpu:    4096,
		},
		ConvertResourceToFirOutput: ecs.FreeInstanceResource{
			InstanceId:       "i-test",
			AvailabilityZone: "us-east-1a",
			Status:           "ACTIVE",
			FreeMemory:       2048,
			FreeCpu:          1024,
		},
		GetInstanceResourcesOutputFree: []ecs.FreeInstanceResource{
			{
				InstanceId:       "i-test",
				AvailabilityZone: "eu-east-1a",
				Status:           "ACTIVE",
				FreeMemory:       6144,
				FreeCpu:          1024,
			},
		},
		DescribeServicesWithOptionsOutput: []service.RunningService{
			{
				ServiceName:  "test-service-1",
				ClusterName:  "testCluster",
				RunningCount: 1,
				PendingCount: 1,
				DesiredCount: 2,
				Status:       "ACTIVE",
				PlacementStrategy: []*ecsService.PlacementStrategy{
					{
						Type:  aws.String("x86_64"),
						Field: aws.String("ecs.cpu-architecture"),
					},
				},
				Events: []service.RunningServiceEvent{
					{
						CreatedAt: time.Now(),
						Id:        "1-2-3-4",
						Message:   "... was unable to place a task because no container instance met all of its requirements ... has insufficient ...",
					},
				},
			},
		},
	}
	s := &MockService{
		GetServicesOutput: &service.DynamoServices{
			ServiceName: "__SERVICES",
			Services: []*service.DynamoServicesElement{
				{
					C: "testCluster",
					S: "test-service1",
				},
			},
		},
		PutClusterInfoOutput: &service.DynamoCluster{},
		GetClusterInfoOutput: &service.DynamoCluster{},
	}
	ma := &MockAutoScaling{}
	as := AutoscalingController{}
	as.startAutoscalingPollingStrategy(0, e, s, ma)
	if s.PutClusterInfoCounter != 1 {
		t.Errorf("s.PutClusterInfoCounter is not 1, didn't went through scaling operation")
	}
	if s.GetClusterInfoOutput.ScalingOperation.Action != "up" {
		t.Errorf("Expected scaling operation to be 'up', got %s", s.GetClusterInfoOutput.ScalingOperation.Action)
	}
}
