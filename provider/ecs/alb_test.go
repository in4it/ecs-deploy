package ecs

import (
	"fmt"
	"testing"

	"github.com/in4it/ecs-deploy/util"
)

func TestGetHighestRule(t *testing.T) {
	if accountId == nil {
		t.Skip(noAWSMsg)
	}
	a, err := newALB(util.GetEnv("TEST_CLUSTERNAME", "test-cluster"))
	if err != nil {
		t.Errorf("Error: %v", err)
		return
	}
	highest, err := a.getHighestRule()
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	fmt.Printf("Highest rule in ALB (%v) is: %d ", a.loadBalancerName, highest)
}
