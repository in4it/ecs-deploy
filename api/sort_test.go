package api

import (
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/in4it/ecs-deploy/service"
)

func TestRuleConditionSort(t *testing.T) {
	conditions := []*service.DeployRuleConditions{
		{
			Hostname: "test",
		},
		{
			Hostname:    "test",
			PathPattern: "/api",
		},
		{
			Hostname:    "test",
			PathPattern: "/api/v1",
		},
	}
	conditionsSorted := []*service.DeployRuleConditions{
		{
			Hostname:    "test",
			PathPattern: "/api/v1",
		},
		{
			Hostname:    "test",
			PathPattern: "/api",
		},
		{
			Hostname: "test",
		},
	}
	sort.Sort(ruleConditionSort(conditions))

	if !cmp.Equal(conditions, conditionsSorted) {
		t.Errorf("Conditions is not correctly sorted")
	}
}
