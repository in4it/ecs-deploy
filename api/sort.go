package api

import (
	"github.com/in4it/ecs-deploy/service"
)

type ruleConditionSort []*service.DeployRuleConditions

func (s ruleConditionSort) Len() int {
	return len(s)
}
func (s ruleConditionSort) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s ruleConditionSort) Less(i, j int) bool {
	return len(s[i].PathPattern) > len(s[j].PathPattern)
}
