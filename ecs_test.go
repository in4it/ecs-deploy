package main

import (
	"testing"
)

func TestWaitUntilServicesStable(t *testing.T) {
	if accountId == nil {
		t.Skip(noAWSMsg)
	}
	ecs := ECS{clusterName: getEnv("TEST_CLUSTERNAME", "test-cluster")}
	err := ecs.waitUntilServicesStable(getEnv("TEST_SERVICENAME", "ecs-deploy"))
	if err != nil {
		t.Errorf("Error: %v", err)
		return
	}
}
