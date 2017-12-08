package main

import (
	"fmt"
	"testing"
)

func TestGetHighestRule(t *testing.T) {
	if accountId == nil {
		t.Skip(noAWSMsg)
	}
	a, err := newALB(getEnv("TEST_CLUSTERNAME", "test-cluster"))
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
