package service

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/in4it/ecs-deploy/util"
)

var accountId *string

const noAWSMsg = "AWS Credentials not found - test skipped"

func TestGetLastDeploy(t *testing.T) {
	if accountId == nil {
		t.Skip(noAWSMsg)
	}
	service := NewService()
	service.ServiceName = util.GetEnv("TEST_SERVICENAME", "ecs-deploy")
	dd, err := service.GetLastDeploy()
	if err != nil {
		if !strings.HasPrefix(err.Error(), "NoItemsFound") {
			t.Errorf("GetLastDeploys: %v", err)
		}
	}
	if dd != nil {
		fmt.Printf("GetLastDeploy: Retrieved last record: %v_%v\n", dd.ServiceName, dd.Time)
	} else {
		fmt.Println("GetLastDeploy: No items found")
	}
}

func TestGetDeploymentByMonth(t *testing.T) {
	if accountId == nil {
		t.Skip(noAWSMsg)
	}
	limit := 20
	service := NewService()
	dds, err := service.GetDeploys("byMonth", int64(limit))
	if err != nil {
		t.Errorf("GetDeploys byMonth: %v", err)
	}
	if len(dds) > limit {
		t.Errorf("GetDeploys byMonth: result higher than limit")
	}
	fmt.Printf("GetDeploys byMonth: retrieved %d records\n", len(dds))
}
func TestGetDeploymentByDay(t *testing.T) {
	if accountId == nil {
		t.Skip(noAWSMsg)
	}
	limit := 20
	service := NewService()
	dds, err := service.GetDeploys("byDay", int64(limit))
	if err != nil {
		t.Errorf("GetDeploys byDay: %v", err)
	}
	if len(dds) > limit {
		t.Errorf("GetDeploys byDay: result higher than limit")
	}
	fmt.Printf("GetDeploys byDay: retrieved %d records\n", len(dds))
}
func TestGetDeploymentSecondToLast(t *testing.T) {
	if accountId == nil {
		t.Skip(noAWSMsg)
	}
	service := NewService()
	service.ServiceName = util.GetEnv("TEST_SERVICENAME", "ecs-deploy")
	dds, err := service.GetDeploys("secondToLast", 1)
	if err != nil {
		if !strings.HasPrefix(err.Error(), "NoSecondToLast") {
			t.Errorf("GetDeploys secondToLast: %v", err)
		}
	}
	if len(dds) > 1 {
		t.Errorf("GetDeploys secondToLast: result higher than 1")
	}
	if len(dds) == 1 {
		fmt.Printf("Retrieved second to last record: %v_%v\n", dds[0].ServiceName, dds[0].Time)
	}
}
func TestGetServices(t *testing.T) {
	if accountId == nil {
		t.Skip(noAWSMsg)
	}
	service := NewService()
	var ds DynamoServices
	err := service.GetServices(&ds)
	if err != nil {
		t.Errorf("Couldn't retrieve services from dynamodb: %v\n", err.Error())
	}
}
func TestGetClusterInfo(t *testing.T) {
	if accountId == nil {
		t.Skip(noAWSMsg)
	}
	service := NewService()
	dc, err := service.GetClusterInfo()
	if err != nil {
		t.Errorf("ClusterInfo: %v", err)
	}
	if dc != nil {
		fmt.Printf("ClusterInfo: Retrieved last record: %v\n", dc.Time)
	} else {
		fmt.Println("ClusterInfo: No items found")
	}
}
func TestGetScalingActivity(t *testing.T) {
	if accountId == nil {
		t.Skip(noAWSMsg)
	}
	service := NewService()

	clusterName := util.GetEnv("TEST_CLUSTERNAME", "testcluster")
	startTime := time.Now().Add(-5 * time.Minute)

	result, _, err := service.GetScalingActivity(clusterName, startTime)
	if err != nil {
		t.Errorf("ScalingActivity: %v", err)
	}
	if result != "" {
		fmt.Printf("Scaling action within last 5 min: %v\n", result)
	} else {
		fmt.Println("No scaling actions within last 5 min")
	}
}
