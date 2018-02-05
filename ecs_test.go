package ecsdeploy

import (
	"testing"
)

func TestWaitUntilServicesStable(t *testing.T) {
	if accountId == nil {
		t.Skip(noAWSMsg)
	}
	ecs := ECS{}
	err := ecs.waitUntilServicesStable(getEnv("TEST_CLUSTERNAME", "test-cluster"), getEnv("TEST_SERVICENAME", "ecs-deploy"), 10)
	if err != nil {
		t.Errorf("Error: %v", err)
		return
	}
}
