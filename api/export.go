package api

import (
	"encoding/base64"
	"errors"
	"io/ioutil"
	"sort"
	"strconv"
	"strings"

	"github.com/in4it/ecs-deploy/provider/ecs"
	"github.com/in4it/ecs-deploy/service"
	"github.com/in4it/ecs-deploy/util"
	"github.com/juju/loggo"
)

// logging
var exportLogger = loggo.GetLogger("export")

type ExportedApps map[string]string

type Export struct {
	templateMap map[string]string
	deployData  *service.Deploy
	alb         map[string]*ecs.ALB
	p           ecs.Paramstore
}

type RulePriority []int64

func (a RulePriority) Len() int           { return len(a) }
func (a RulePriority) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a RulePriority) Less(i, j int) bool { return a[i] < a[j] }

type ListenerRuleExport struct {
	RuleKeys RulePriority           `json:"ruleKeys" binding:"dive"`
	Rules    map[int64]ListenerRule `json:"rules" binding:"dive"`
}

type ListenerRule struct {
	ListenerRuleArn string                  `json:"listenerRuleArn"`
	TargetGroupArn  string                  `json:"targetGroupArn" binding:"dive"`
	Conditions      []ListenerRuleCondition `json:"conditions" binding:"dive"`
}
type ListenerRuleCondition struct {
	Field  string `json:"field" binding:"dive"`
	Values string `json:"values" binding:"dive"`
}

func (e *Export) getTemplateMap(serviceName, clusterName string) error {
	// retrieve data
	iam := ecs.IAM{}
	err := iam.GetAccountId()
	if err != nil {
		return err
	}
	// get deployment obj
	s := service.NewService()
	s.ServiceName = serviceName
	s.ClusterName = clusterName

	dd, err := s.GetLastDeploy()
	if err != nil {
		return err
	}
	if dd.DeployData == nil {
		return errors.New("DeployData is empty")
	}
	e.deployData = dd.DeployData
	exportLogger.Debugf("got: %+v", e.deployData)

	// retrieve alb data
	var loadBalancer string
	if e.deployData.LoadBalancer == "" {
		loadBalancer = clusterName
	} else {
		loadBalancer = e.deployData.LoadBalancer
	}
	if _, ok := e.alb[loadBalancer]; !ok {
		e.alb[loadBalancer], err = ecs.NewALB(loadBalancer)
		if err != nil {
			return err
		}
		// get rules for all listener
		err = e.alb[loadBalancer].GetRulesForAllListeners()
		if err != nil {
			return err
		}
	}

	// get target group (if service has loadbalancer)
	var targetGroup *string
	if strings.ToLower(e.deployData.ServiceProtocol) != "none" {
		targetGroup, err = e.alb[loadBalancer].GetTargetGroupArn(serviceName)
		if err != nil {
			return err
		}
		if targetGroup == nil {
			return errors.New("No target group found for " + serviceName)
		}
	}

	// init map
	e.templateMap = make(map[string]string)
	e.templateMap["${SERVICE}"] = serviceName
	e.templateMap["${CLUSTERNAME}"] = clusterName
	e.templateMap["${LOADBALANCER}"] = loadBalancer
	if targetGroup != nil {
		e.templateMap["${TARGET_GROUP_ARN}"] = *targetGroup
	}
	e.templateMap["${SERVICE_DESIREDCOUNT}"] = strconv.FormatInt(e.deployData.DesiredCount, 10)
	if e.deployData.MinimumHealthyPercent == 0 {
		e.templateMap["${SERVICE_MINIMUMHEALTHYPERCENT}"] = "// no minimum healthy percent set"
	} else {
		e.templateMap["${SERVICE_MINIMUMHEALTHYPERCENT}"] = `deployment_minimum_healthy_percent = "` + strconv.FormatInt(e.deployData.MinimumHealthyPercent, 10) + `"`
	}
	if e.deployData.MaximumPercent == 0 {
		e.templateMap["${SERVICE_MAXIMUMPERCENT}"] = "// no maximum percent set"
	} else {
		e.templateMap["${SERVICE_MAXIMUMPERCENT}"] = `deployment_maximum_percent = "` + strconv.FormatInt(e.deployData.MaximumPercent, 10) + `"`
	}
	e.templateMap["${SERVICE_PORT}"] = strconv.FormatInt(e.deployData.ServicePort, 10)
	e.templateMap["${SERVICE_PROTOCOL}"] = e.deployData.ServiceProtocol
	e.templateMap["${AWS_REGION}"] = util.GetEnv("AWS_REGION", "")
	e.templateMap["${ACCOUNT_ID}"] = iam.AccountId
	e.templateMap["${PARAMSTORE_PREFIX}"] = util.GetEnv("PARAMSTORE_PREFIX", "")
	if dd.DeployData.EnvNamespace == "" {
		e.templateMap["${NAMESPACE}"] = serviceName
	} else {
		e.templateMap["${NAMESPACE}"] = dd.DeployData.EnvNamespace
	}
	e.templateMap["${AWS_ACCOUNT_ENV}"] = util.GetEnv("AWS_ACCOUNT_ENV", "")
	e.templateMap["${PARAMSTORE_KMS_ARN}"] = util.GetEnv("PARAMSTORE_KMS_ARN", "")
	e.templateMap["${VPC_ID}"] = e.alb[loadBalancer].VpcId
	if e.deployData.HealthCheck.HealthyThreshold != 0 {
		b, err := ioutil.ReadFile("templates/export/alb_targetgroup_healthcheck.tf")
		if err != nil {
			exportLogger.Errorf("Can't read template templates/export/alb_targetgroup_healthcheck.tf")
			return err
		}
		str := string(b)
		if e.deployData.HealthCheck.HealthyThreshold != 0 {
			str = strings.Replace(str, "${HEALTHCHECK_HEALTHYTHRESHOLD}", strconv.FormatInt(e.deployData.HealthCheck.HealthyThreshold, 10), -1)
		} else {
			str = strings.Replace(str, "${HEALTHCHECK_HEALTHYTHRESHOLD}", "3", -1)
		}
		if e.deployData.HealthCheck.UnhealthyThreshold != 0 {
			str = strings.Replace(str, "${HEALTHCHECK_UNHEALTHYTHRESHOLD}", strconv.FormatInt(e.deployData.HealthCheck.UnhealthyThreshold, 10), -1)
		} else {
			str = strings.Replace(str, "${HEALTHCHECK_UNHEALTHYTHRESHOLD}", "2", -1)
		}
		if e.deployData.HealthCheck.Protocol != "" {
			str = strings.Replace(str, "${HEALTHCHECK_PROTOCOL}", e.deployData.HealthCheck.Protocol, -1)
		} else {
			str = strings.Replace(str, "${HEALTHCHECK_PROTOCOL}", "HTTP", -1)
		}
		if e.deployData.HealthCheck.Path != "" {
			str = strings.Replace(str, "${HEALTHCHECK_PATH}", e.deployData.HealthCheck.Path, -1)
		} else {
			str = strings.Replace(str, "${HEALTHCHECK_PATH}", "/", -1)
		}
		if e.deployData.HealthCheck.Interval != 0 {
			str = strings.Replace(str, "${HEALTHCHECK_INTERVAL}", strconv.FormatInt(e.deployData.HealthCheck.Interval, 10), -1)
		} else {
			str = strings.Replace(str, "${HEALTHCHECK_INTERVAL}", "30", -1)
		}
		if e.deployData.HealthCheck.Matcher != "" {
			str = strings.Replace(str, "${HEALTHCHECK_MATCHER}", e.deployData.HealthCheck.Matcher, -1)
		} else {
			str = strings.Replace(str, "${HEALTHCHECK_MATCHER}", "200", -1)
		}
		if e.deployData.HealthCheck.Timeout > 0 {
			str = strings.Replace(str, "${HEALTHCHECK_TIMEOUT}", strconv.FormatInt(e.deployData.HealthCheck.Timeout, 10), -1)
		} else {
			str = strings.Replace(str, "${HEALTHCHECK_TIMEOUT}", "5", -1)
		}
		e.templateMap["${HEALTHCHECK}"] = str
	}
	return nil
}

// check first whether the template is in the parameter store
// if not, use the default template from the template path
func (e *Export) getTemplate(template string) (*string, error) {
	parameter, ok := e.p.Parameters["TEMPLATES_EXPORT_"+strings.Replace(strings.ToUpper(template), ".", "_", -1)]
	str := parameter.Value
	if !ok {
		b, err := ioutil.ReadFile("templates/export/" + template)
		if err != nil {
			exportLogger.Errorf("Can't read template templates/export/" + template)
			return nil, err
		}
		str = string(b)
	}
	// replace
	for k, v := range e.templateMap {
		str = strings.Replace(str, k, v, -1)
	}
	return &str, nil
}

func (e *Export) terraform() (*map[string]ExportedApps, error) {
	// get all services
	export := make(map[string]ExportedApps)
	export["apps"] = make(ExportedApps)
	e.alb = make(map[string]*ecs.ALB)

	var ds service.DynamoServices
	// get possible parameters
	e.p = ecs.Paramstore{}
	e.p.GetParameters(e.p.GetPrefix(), true)
	// ecr obj
	ecr := ecs.ECR{}
	// get services
	s := service.NewService()
	err := s.GetServices(&ds)
	if err != nil {
		return nil, err
	}
	for _, service := range ds.Services {
		var ret string
		err := e.getTemplateMap(service.S, service.C)
		if err != nil {
			return nil, err
		}
		exportLogger.Debugf("Retrieved template map: %+v", e.templateMap)

		// check if we have targetGroup
		var processTargetGroup bool
		if _, ok := e.templateMap["${TARGET_GROUP_ARN}"]; ok {
			processTargetGroup = true
		}
		// check whether to process ecr
		processEcr, err := ecr.RepositoryExists(service.S)
		if err != nil {
			return nil, err
		}

		var toProcess []string

		if processEcr {
			toProcess = append(toProcess, "ecr")
		}
		if processTargetGroup {
			toProcess = append(toProcess, []string{"ecs", "iam", "alb_targetgroup"}...)
		} else {
			toProcess = append(toProcess, []string{"ecs", "iam"}...)
		}
		if e.p.IsEnabled() {
			toProcess = append(toProcess, "iam_paramstore")
		}
		for _, v := range toProcess {
			t, err := e.getTemplate(v + ".tf")
			if err != nil {
				return nil, err
			}
			ret += *t
		}

		// get listener rules
		if processTargetGroup {
			t, err := e.getListenerRules(service.S, service.C, service.Listeners, e.templateMap["${LOADBALANCER}"])
			if err != nil {
				return nil, err
			}
			ret += *t
		}
		export["apps"][service.S] = base64.StdEncoding.EncodeToString([]byte(ret))
	}
	return &export, nil
}

func (e *Export) getListenerRules(serviceName string, clusterName string, listeners []string, loadBalancer string) (*string, error) {
	var ret string
	// listeners
	albListenerRule, err := e.getTemplate("alb_listenerrule.tf")
	if err != nil {
		return nil, err
	}
	condition, err := e.getTemplate("alb_listenerrule_condition.tf")
	if err != nil {
		return nil, err
	}
	if len(e.deployData.RuleConditions) == 0 {
		exportLogger.Debugf("No rule conditions, going with default rules")
		for _, l := range listeners {
			a := strings.Replace(*albListenerRule, "${LISTENER_ARN}", l, -1)
			for _, v := range []string{"/" + serviceName, "/" + serviceName + "/*"} {
				// get priority
				ruleArn, priority, err := e.alb[loadBalancer].FindRule(l, e.templateMap["${TARGET_GROUP_ARN}"], []string{"path-pattern"}, []string{v})
				if err != nil {
					return nil, err
				}
				// replace listeners and return template
				a = strings.Replace(a, "${LISTENER_PRIORITY}", *priority, -1)
				a = strings.Replace(a, "${LISTENER_RULE_ARN}", *ruleArn, -1)
				c := strings.Replace(*condition, "${LISTENER_CONDITION_FIELD}", "path-pattern", -1)
				c = strings.Replace(c, "${LISTENER_CONDITION_VALUE}", v, -1)
				ret += strings.Replace(a, "${LISTENER_CONDITION_RULE}", c, -1)
			}
		}
	} else {
		exportLogger.Debugf("Found rule conditions in deploy, examining conditions")
		for _, y := range e.deployData.RuleConditions {
			for _, l := range e.alb[loadBalancer].Listeners {
				for _, l2 := range y.Listeners {
					if l.Protocol != nil && strings.ToLower(*l.Protocol) == strings.ToLower(l2) {
						a := strings.Replace(*albListenerRule, "${LISTENER_ARN}", *l.ListenerArn, -1)
						var c, cc string
						var f []string
						var v []string
						if y.PathPattern != "" {
							f = append(f, "path-pattern")
							v = append(v, y.PathPattern)
							c = strings.Replace(*condition, "${LISTENER_CONDITION_FIELD}", "path-pattern", -1)
							c = strings.Replace(c, "${LISTENER_CONDITION_VALUE}", y.PathPattern, -1)
						}
						if y.Hostname != "" {
							f = append(f, "host-header")
							v = append(v, y.Hostname+"."+e.alb[loadBalancer].GetDomain())
							cc = strings.Replace(*condition, "${LISTENER_CONDITION_FIELD}", "host-header", -1)
							cc = strings.Replace(cc, "${LISTENER_CONDITION_VALUE}", y.Hostname+"."+e.alb[loadBalancer].GetDomain(), -1)
						}
						// get priority
						ruleArn, priority, err := e.alb[loadBalancer].FindRule(*l.ListenerArn, e.templateMap["${TARGET_GROUP_ARN}"], f, v)
						if err != nil {
							return nil, err
						}
						a = strings.Replace(a, "${LISTENER_PRIORITY}", *priority, -1)
						a = strings.Replace(a, "${LISTENER_RULE_ARN}", *ruleArn, -1)
						// get everything together and return template
						ret += strings.Replace(a, "${LISTENER_CONDITION_RULE}", c+cc, -1)
					}
				}
			}
		}
	}
	return &ret, nil
}

func (e *Export) getTargetGroupArn(serviceName string) (*string, error) {
	a := ecs.ALB{}
	return a.GetTargetGroupArn(serviceName)
}
func (e *Export) getListenerRuleArn(serviceName string, rulePriority string) (*string, error) {
	var clusterName string
	var listenerRuleArn string
	var ds service.DynamoServices
	s := service.NewService()
	s.GetServices(&ds)
	for _, service := range ds.Services {
		if service.S == serviceName {
			clusterName = service.C
		}
	}
	if clusterName == "" {
		return nil, errors.New("Service not found: " + serviceName)
	}
	a, err := ecs.NewALB(clusterName)
	if err != nil {
		return nil, err
	}
	targetGroupArn, err := a.GetTargetGroupArn(serviceName)
	if err != nil {
		return nil, err
	}
	a.GetRulesForAllListeners()
	for _, rules := range a.Rules {
		for _, rule := range rules {
			if *rule.Priority == rulePriority {
				if listenerRuleArn != "" {
					return nil, errors.New("Duplicate listener rule found, can't determine listener (rule = " + rulePriority + ", Conflict between " + listenerRuleArn + " and " + *rule.RuleArn + ")")
				} else {
					if len(rule.Actions) > 0 && *rule.Actions[0].TargetGroupArn == *targetGroupArn {
						listenerRuleArn = *rule.RuleArn
					}
				}
			}
		}
	}
	if listenerRuleArn == "" {
		return nil, errors.New("No rule with priority " + rulePriority + " found")
	}
	return &listenerRuleArn, nil
}
func (e *Export) getListenerRuleArns(serviceName string) (*ListenerRuleExport, error) {
	var clusterName string
	var ds service.DynamoServices
	var result *ListenerRuleExport
	var exportRuleKeys RulePriority
	exportRules := make(map[int64]ListenerRule)
	s := service.NewService()
	s.GetServices(&ds)
	for _, service := range ds.Services {
		if service.S == serviceName {
			clusterName = service.C
		}
	}
	if clusterName == "" {
		return nil, errors.New("Service not found: " + serviceName)
	}
	a, err := ecs.NewALB(clusterName)
	if err != nil {
		return nil, err
	}
	targetGroupArn, err := a.GetTargetGroupArn(serviceName)
	if err != nil {
		return nil, err
	}
	a.GetRulesForAllListeners()
	for _, rules := range a.Rules {
		for _, rule := range rules {
			if len(rule.Actions) > 0 && *rule.Actions[0].TargetGroupArn == *targetGroupArn {
				priority, err := strconv.ParseInt(*rule.Priority, 10, 64)
				if err != nil {
					return nil, err
				}
				var conditions []ListenerRuleCondition
				for _, condition := range rule.Conditions {
					if len(condition.Values) > 0 {
						conditions = append(conditions, ListenerRuleCondition{Field: *condition.Field, Values: *condition.Values[0]})
					}
				}
				exportRuleKeys = append(exportRuleKeys, priority)
				exportRules[priority] = ListenerRule{
					ListenerRuleArn: *rule.RuleArn,
					TargetGroupArn:  *rule.Actions[0].TargetGroupArn,
					Conditions:      conditions,
				}
			}
		}
	}
	if len(exportRuleKeys) == 0 {
		return nil, errors.New("No rules found for service: " + serviceName)
	}
	sort.Sort(exportRuleKeys)
	result = &ListenerRuleExport{RuleKeys: exportRuleKeys, Rules: exportRules}
	return result, nil
}
