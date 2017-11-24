package main

import (
	"encoding/base64"
	"errors"
	"github.com/juju/loggo"
	"io/ioutil"
	"strconv"
	"strings"
)

// logging
var exportLogger = loggo.GetLogger("export")

type ExportedApps map[string]string

type Export struct {
	templateMap map[string]string
	deployData  *Deploy
	alb         ALB
}

func (e *Export) getTemplateMap(serviceName, clusterName string) error {
	// retrieve data
	iam := IAM{}
	err := iam.getAccountId()
	if err != nil {
		return err
	}
	// retrieve data
	e.alb = ALB{}
	err = e.alb.init(clusterName)
	if err != nil {
		return err
	}
	// get rules for all listener
	err = e.alb.getRulesForAllListeners()
	if err != nil {
		return err
	}
	// get target group
	targetGroup, err := e.alb.getTargetGroupArn(serviceName)
	if err != nil {
		return err
	}
	if targetGroup == nil {
		return errors.New("No target group found for " + serviceName)
	}
	// get deployment obj
	s := Service{serviceName: serviceName, clusterName: clusterName}
	dd, err := s.getLastDeploy()
	if err != nil {
		return err
	}
	if dd.DeployData == nil {
		return errors.New("DeployData is empty")
	}
	e.deployData = dd.DeployData
	exportLogger.Debugf("got: %+v", e.deployData)
	// init map
	e.templateMap = make(map[string]string)
	e.templateMap["${SERVICE}"] = serviceName
	e.templateMap["${CLUSTERNAME}"] = clusterName
	e.templateMap["${TARGET_GROUP_ARN}"] = *targetGroup
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
	e.templateMap["${AWS_REGION}"] = getEnv("AWS_REGION", "")
	e.templateMap["${ACCOUNT_ID}"] = iam.accountId
	e.templateMap["${PARAMSTORE_PREFIX}"] = getEnv("PARAMSTORE_PREFIX", "")
	e.templateMap["${AWS_ACCOUNT_ENV}"] = getEnv("AWS_ACCOUNT_ENV", "")
	e.templateMap["${PARAMSTORE_KMS_ARN}"] = getEnv("PARAMSTORE_KMS_ARN", "")
	e.templateMap["${VPC_ID}"] = e.alb.vpcId
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

func (e *Export) getTemplate(template string) (*string, error) {
	b, err := ioutil.ReadFile("templates/export/" + template)
	if err != nil {
		exportLogger.Errorf("Can't read template templates/export/" + template)
		return nil, err
	}
	str := string(b)
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

	var ds DynamoServices
	p := Paramstore{}
	s := Service{}
	err := s.getServices(&ds)
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

		toProcess := []string{"ecr", "ecs", "iam", "alb_targetgroup"}
		if p.isEnabled() {
			toProcess = append(toProcess, "iam_paramstore")
		}
		for _, v := range toProcess {
			t, err := e.getTemplate(v + ".tf")
			if err != nil {
				return nil, err
			}
			ret += *t
		}

		t, err := e.getListenerRules(service.S, service.C, service.L)
		if err != nil {
			return nil, err
		}
		ret += *t
		export["apps"][service.S] = base64.StdEncoding.EncodeToString([]byte(ret))
	}
	return &export, nil
}

func (e *Export) getListenerRules(serviceName string, clusterName string, listeners []string) (*string, error) {
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
				priority, err := e.alb.findRulePriority(l, e.templateMap["${TARGET_GROUP_ARN}"], []string{"path-pattern"}, []string{v})
				if err != nil {
					return nil, err
				}
				// replace listeners and return template
				a = strings.Replace(a, "${LISTENER_PRIORITY}", *priority, -1)
				c := strings.Replace(*condition, "${LISTENER_CONDITION_FIELD}", "path-pattern", -1)
				c = strings.Replace(c, "${LISTENER_CONDITION_VALUE}", v, -1)
				ret += strings.Replace(a, "${LISTENER_CONDITION_RULE}", c, -1)
			}
		}
	} else {
		exportLogger.Debugf("Found rule conditions in deploy, examining conditions")
		for _, y := range e.deployData.RuleConditions {
			for _, l := range e.alb.listeners {
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
							v = append(v, y.Hostname+"."+e.alb.getDomain())
							cc = strings.Replace(*condition, "${LISTENER_CONDITION_FIELD}", "host-header", -1)
							cc = strings.Replace(cc, "${LISTENER_CONDITION_VALUE}", y.Hostname+"."+e.alb.getDomain(), -1)
						}
						// get priority
						priority, err := e.alb.findRulePriority(*l.ListenerArn, e.templateMap["${TARGET_GROUP_ARN}"], f, v)
						if err != nil {
							return nil, err
						}
						a = strings.Replace(a, "${LISTENER_PRIORITY}", *priority, -1)
						// get everything together and return template
						ret += strings.Replace(a, "${LISTENER_CONDITION_RULE}", c+cc, -1)
					}
				}
			}
		}
	}
	return &ret, nil
}
