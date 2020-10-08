package api

import (
	"testing"

	"github.com/in4it/ecs-deploy/service"
)

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
