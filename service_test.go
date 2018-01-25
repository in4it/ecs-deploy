package main

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestGetLastDeploy(t *testing.T) {
	if accountId == nil {
		t.Skip(noAWSMsg)
	}
	service := newService()
	service.serviceName = getEnv("TEST_SERVICENAME", "ecs-deploy")
	dd, err := service.getLastDeploy()
	if err != nil {
		if !strings.HasPrefix(err.Error(), "NoItemsFound") {
			t.Errorf("getLastDeploys: %v", err)
		}
	}
	if dd != nil {
		fmt.Printf("getLastDeploy: Retrieved last record: %v_%v\n", dd.ServiceName, dd.Time)
	} else {
		fmt.Println("getLastDeploy: No items found")
	}
}

func TestGetDeploymentByMonth(t *testing.T) {
	if accountId == nil {
		t.Skip(noAWSMsg)
	}
	limit := 20
	service := newService()
	dds, err := service.getDeploys("byMonth", int64(limit))
	if err != nil {
		t.Errorf("getDeploys byMonth: %v", err)
	}
	if len(dds) > limit {
		t.Errorf("getDeploys byMonth: result higher than limit")
	}
	fmt.Printf("getDeploys byMonth: retrieved %d records\n", len(dds))
}
func TestGetDeploymentByDay(t *testing.T) {
	if accountId == nil {
		t.Skip(noAWSMsg)
	}
	limit := 20
	service := newService()
	dds, err := service.getDeploys("byDay", int64(limit))
	if err != nil {
		t.Errorf("getDeploys byDay: %v", err)
	}
	if len(dds) > limit {
		t.Errorf("getDeploys byDay: result higher than limit")
	}
	fmt.Printf("getDeploys byDay: retrieved %d records\n", len(dds))
}
func TestGetDeploymentSecondToLast(t *testing.T) {
	if accountId == nil {
		t.Skip(noAWSMsg)
	}
	service := newService()
	service.serviceName = getEnv("TEST_SERVICENAME", "ecs-deploy")
	dds, err := service.getDeploys("secondToLast", 1)
	if err != nil {
		if !strings.HasPrefix(err.Error(), "NoSecondToLast") {
			t.Errorf("getDeploys secondToLast: %v", err)
		}
	}
	if len(dds) > 1 {
		t.Errorf("getDeploys secondToLast: result higher than 1")
	}
	if len(dds) == 1 {
		fmt.Printf("Retrieved second to last record: %v_%v\n", dds[0].ServiceName, dds[0].Time)
	}
}
func TestGetServices(t *testing.T) {
	if accountId == nil {
		t.Skip(noAWSMsg)
	}
	service := newService()
	var ds DynamoServices
	err := service.getServices(&ds)
	if err != nil {
		t.Errorf("Couldn't retrieve services from dynamodb: %v\n", err.Error())
	}
}
func TestGetClusterInfo(t *testing.T) {
	if accountId == nil {
		t.Skip(noAWSMsg)
	}
	service := newService()
	dc, err := service.getClusterInfo()
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
	service := newService()

	clusterName := getEnv("TEST_CLUSTERNAME", "testcluster")
	startTime := time.Now().Add(-5 * time.Minute)

	result, err := service.getScalingActivity(clusterName, startTime)
	if err != nil {
		t.Errorf("ScalingActivity: %v", err)
	}
	if result != "" {
		fmt.Printf("Scaling action within last 5 min: %v\n", result)
	} else {
		fmt.Println("No scaling actions within last 5 min")
	}
}
