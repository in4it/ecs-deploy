package ecs

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/acm"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/in4it/ecs-deploy/service"
	"github.com/in4it/ecs-deploy/util"
	"github.com/juju/loggo"

	"errors"
	"strconv"
	"strings"
)

// logging
var albLogger = loggo.GetLogger("alb")

// ALB struct
type ALB struct {
	loadBalancerName string
	loadBalancerArn  string
	VpcId            string
	Listeners        []*elbv2.Listener
	Domain           string
	Rules            map[string][]*elbv2.Rule
	DnsName          string
}

func NewALB(loadBalancerName string) (*ALB, error) {
	a := ALB{}
	a.loadBalancerName = loadBalancerName
	// retrieve vpcId and loadBalancerArn
	svc := elbv2.New(session.New())
	input := &elbv2.DescribeLoadBalancersInput{
		Names: []*string{
			aws.String(loadBalancerName),
		},
	}

	result, err := svc.DescribeLoadBalancers(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case elbv2.ErrCodeLoadBalancerNotFoundException:
				albLogger.Errorf(elbv2.ErrCodeLoadBalancerNotFoundException+": %v", aerr.Error())
			default:
				albLogger.Errorf(aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			albLogger.Errorf(err.Error())
		}
		return nil, errors.New("Could not describe loadbalancer")
	} else if len(result.LoadBalancers) == 0 {
		return nil, errors.New("Could not describe loadbalancer (no elements returned)")
	}
	a.loadBalancerArn = *result.LoadBalancers[0].LoadBalancerArn
	a.loadBalancerName = *result.LoadBalancers[0].LoadBalancerName
	a.VpcId = *result.LoadBalancers[0].VpcId

	// get listeners
	err = a.GetListeners()
	if err != nil {
		return nil, err
	} else if len(result.LoadBalancers) == 0 {
		return nil, errors.New("Could not get listeners for loadbalancer (no elements returned)")
	}
	// get domain (if SSL cert is attached)
	err = a.GetDomainUsingCertificate()
	if err != nil {
		return nil, err
	}

	return &a, nil
}

// get the listeners for the loadbalancer
func NewALBAndCreate(loadBalancerName, ipAddressType string, scheme string, securityGroups []string, subnets []string, lbType string) (*ALB, error) {
	a := ALB{}
	svc := elbv2.New(session.New())
	input := &elbv2.CreateLoadBalancerInput{
		IpAddressType:  aws.String(ipAddressType),
		Name:           aws.String(loadBalancerName),
		Scheme:         aws.String(scheme),
		SecurityGroups: aws.StringSlice(securityGroups),
		Subnets:        aws.StringSlice(subnets),
		Type:           aws.String(lbType),
	}

	result, err := svc.CreateLoadBalancer(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			albLogger.Errorf(aerr.Error())
			return nil, aerr
		}
		albLogger.Errorf(err.Error())
		return nil, err
	}
	if len(result.LoadBalancers) == 0 {
		return nil, errors.New("No loadbalancers returned")
	}
	a.loadBalancerArn = aws.StringValue(result.LoadBalancers[0].LoadBalancerArn)
	a.DnsName = aws.StringValue(result.LoadBalancers[0].DNSName)
	a.VpcId = aws.StringValue(result.LoadBalancers[0].VpcId)
	return &a, nil
}

func (a *ALB) DeleteLoadBalancer() error {
	svc := elbv2.New(session.New())
	input := &elbv2.DeleteLoadBalancerInput{
		LoadBalancerArn: aws.String(a.loadBalancerArn),
	}
	_, err := svc.DeleteLoadBalancer(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			albLogger.Errorf(aerr.Error())
			return aerr
		}
		albLogger.Errorf(err.Error())
		return err
	}
	return nil
}

func (a *ALB) CreateListener(protocol string, port int64, targetGroupArn string) error {
	// only HTTP is supported for now
	svc := elbv2.New(session.New())
	input := &elbv2.CreateListenerInput{
		LoadBalancerArn: aws.String(a.loadBalancerArn),
		Port:            aws.Int64(port),
		Protocol:        aws.String(protocol),
		DefaultActions: []*elbv2.Action{
			{Type: aws.String("forward"), TargetGroupArn: aws.String(targetGroupArn)},
		},
	}

	result, err := svc.CreateListener(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			albLogger.Errorf(aerr.Error())
		} else {
			albLogger.Errorf(err.Error())
		}
		return err
	}
	if len(result.Listeners) == 0 {
		return errors.New("No listeners returned")
	}
	a.Listeners = append(a.Listeners, result.Listeners[0])
	return nil
}
func (a *ALB) DeleteListener(listenerArn string) error {
	svc := elbv2.New(session.New())
	input := &elbv2.DeleteListenerInput{
		ListenerArn: aws.String(listenerArn),
	}

	_, err := svc.DeleteListener(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			albLogger.Errorf(aerr.Error())
		} else {
			albLogger.Errorf(err.Error())
		}
		return err
	}
	return nil
}

// get the listeners for the loadbalancer
func (a *ALB) GetListeners() error {
	svc := elbv2.New(session.New())
	input := &elbv2.DescribeListenersInput{LoadBalancerArn: aws.String(a.loadBalancerArn)}

	result, err := svc.DescribeListeners(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case elbv2.ErrCodeListenerNotFoundException:
				albLogger.Errorf(elbv2.ErrCodeListenerNotFoundException+": %v", aerr.Error())
			case elbv2.ErrCodeLoadBalancerNotFoundException:
				albLogger.Errorf(elbv2.ErrCodeLoadBalancerNotFoundException+": %v", aerr.Error())
			default:
				albLogger.Errorf(aerr.Error())
			}
		} else {
			albLogger.Errorf(err.Error())
		}
		return errors.New("Could not get Listeners for loadbalancer")
	}
	for _, l := range result.Listeners {
		a.Listeners = append(a.Listeners, l)
	}
	return nil
}

// get the domain using certificates
func (a *ALB) GetDomainUsingCertificate() error {
	svc := acm.New(session.New())
	for _, l := range a.Listeners {
		for _, c := range l.Certificates {
			albLogger.Debugf("ALB Certificate found with arn: %v", *c.CertificateArn)
			input := &acm.DescribeCertificateInput{
				CertificateArn: c.CertificateArn,
			}

			result, err := svc.DescribeCertificate(input)
			if err != nil {
				if aerr, ok := err.(awserr.Error); ok {
					switch aerr.Code() {
					case acm.ErrCodeResourceNotFoundException:
						albLogger.Errorf(acm.ErrCodeResourceNotFoundException+": %v", aerr.Error())
					case acm.ErrCodeInvalidArnException:
						albLogger.Errorf(acm.ErrCodeInvalidArnException+": %v", aerr.Error())
					default:
						albLogger.Errorf(aerr.Error())
					}
				} else {
					albLogger.Errorf(err.Error())
				}
				return errors.New("Could not describe certificate")
			}
			albLogger.Debugf("Domain found through ALB certificate: %v", *result.Certificate.DomainName)
			s := strings.Split(*result.Certificate.DomainName, ".")
			if len(s) >= 2 {
				a.Domain = s[len(s)-2] + "." + s[len(s)-1]
			}
			return nil
		}
	}
	return nil
}

func (a *ALB) CreateTargetGroup(serviceName string, d service.Deploy) (*string, error) {
	svc := elbv2.New(session.New())
	input := &elbv2.CreateTargetGroupInput{
		Name:     aws.String(util.TruncateString(serviceName, 32)),
		VpcId:    aws.String(a.VpcId),
		Port:     aws.Int64(d.ServicePort),
		Protocol: aws.String(d.ServiceProtocol),
	}
	if d.HealthCheck.HealthyThreshold != 0 {
		input.SetHealthyThresholdCount(d.HealthCheck.HealthyThreshold)
	}
	if d.HealthCheck.UnhealthyThreshold != 0 {
		input.SetUnhealthyThresholdCount(d.HealthCheck.UnhealthyThreshold)
	}
	if d.HealthCheck.Path != "" {
		input.SetHealthCheckPath(d.HealthCheck.Path)
	}
	if d.HealthCheck.Port != "" {
		input.SetHealthCheckPort(d.HealthCheck.Port)
	}
	if d.HealthCheck.Protocol != "" {
		input.SetHealthCheckProtocol(d.HealthCheck.Protocol)
	}
	if d.HealthCheck.Interval != 0 {
		input.SetHealthCheckIntervalSeconds(d.HealthCheck.Interval)
	}
	if d.HealthCheck.Matcher != "" {
		input.SetMatcher(&elbv2.Matcher{HttpCode: aws.String(d.HealthCheck.Matcher)})
	}
	if d.HealthCheck.Timeout > 0 {
		input.SetHealthCheckTimeoutSeconds(d.HealthCheck.Timeout)
	}
	if d.NetworkMode == "awsvpc" && len(d.NetworkConfiguration.Subnets) > 0 {
		input.SetTargetType("ip")
	}

	result, err := svc.CreateTargetGroup(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case elbv2.ErrCodeDuplicateTargetGroupNameException:
				albLogger.Errorf(elbv2.ErrCodeDuplicateTargetGroupNameException+": %v", aerr.Error())
			case elbv2.ErrCodeTooManyTargetGroupsException:
				albLogger.Errorf(elbv2.ErrCodeTooManyTargetGroupsException+": %v", aerr.Error())
			case elbv2.ErrCodeInvalidConfigurationRequestException:
				albLogger.Errorf(elbv2.ErrCodeInvalidConfigurationRequestException+": %v", aerr.Error())
			default:
				albLogger.Errorf(aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			albLogger.Errorf(err.Error())
		}
		return nil, errors.New("Could not create target group")
	} else if len(result.TargetGroups) == 0 {
		return nil, errors.New("Could not create target group (target group list is empty)")
	}
	return result.TargetGroups[0].TargetGroupArn, nil
}
func (a *ALB) DeleteTargetGroup(targetGroupArn string) error {
	svc := elbv2.New(session.New())
	input := &elbv2.DeleteTargetGroupInput{
		TargetGroupArn: aws.String(targetGroupArn),
	}
	_, err := svc.DeleteTargetGroup(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			albLogger.Errorf(aerr.Error())
		} else {
			albLogger.Errorf(err.Error())
		}
		return err
	}
	return nil
}

func (a *ALB) GetHighestRule() (int64, error) {
	var highest int64
	svc := elbv2.New(session.New())

	for _, listener := range a.Listeners {
		input := &elbv2.DescribeRulesInput{ListenerArn: listener.ListenerArn}

		c := true // parse more pages if c is true
		result, err := svc.DescribeRules(input)
		for c {
			if err != nil {
				if aerr, ok := err.(awserr.Error); ok {
					switch aerr.Code() {
					case elbv2.ErrCodeListenerNotFoundException:
						albLogger.Errorf(elbv2.ErrCodeListenerNotFoundException+": %v", aerr.Error())
					case elbv2.ErrCodeRuleNotFoundException:
						albLogger.Errorf(elbv2.ErrCodeRuleNotFoundException+": %v", aerr.Error())
					default:
						albLogger.Errorf(aerr.Error())
					}
				} else {
					// Print the error, cast err to awserr.Error to get the Code and
					// Message from an error.
					albLogger.Errorf(err.Error())
				}
				return 0, errors.New("Could not describe alb listener rules")
			}

			albLogger.Tracef("Looping rules: %+v", result.Rules)
			for _, rule := range result.Rules {
				if i, _ := strconv.ParseInt(*rule.Priority, 10, 64); i > highest {
					albLogger.Debugf("Found rule with priority: %d", i)
					highest = i
				}
			}
			if result.NextMarker == nil || len(*result.NextMarker) == 0 {
				c = false
			} else {
				input.SetMarker(*result.NextMarker)
				result, err = svc.DescribeRules(input)
			}
		}
	}

	albLogger.Debugf("Higest rule: %d", highest)

	return highest, nil
}

func (a *ALB) CreateRuleForAllListeners(ruleType string, targetGroupArn string, rules []string, priority int64) ([]string, error) {
	var listeners []string
	for _, l := range a.Listeners {
		err := a.CreateRule(ruleType, *l.ListenerArn, targetGroupArn, rules, priority, service.DeployRuleConditionsCognitoAuth{})
		if err != nil {
			return nil, err
		}
		listeners = append(listeners, *l.ListenerArn)
	}
	return listeners, nil
}

func (a *ALB) CreateRuleForListeners(ruleType string, listeners []string, targetGroupArn string, rules []string, priority int64, cognitoAuth service.DeployRuleConditionsCognitoAuth) ([]string, error) {
	retListeners := a.getListenersArnForProtocol(listeners)
	for proto, listener := range retListeners {
		var err error
		// if cognito is set, a redirect is needed instead (cognito doesn't work with http)
		if proto == "http" && cognitoAuth.ClientName != "" {
			err = a.CreateHTTPSRedirectRule(ruleType, listener, targetGroupArn, rules, priority)
		} else {
			err = a.CreateRule(ruleType, listener, targetGroupArn, rules, priority, cognitoAuth)
		}
		if err != nil {
			return nil, err
		}
	}
	listenerArns := []string{}
	for _, v := range retListeners {
		listenerArns = append(listenerArns, v)
	}
	return listenerArns, nil
}

func (a *ALB) getListenersArnForProtocol(listeners []string) map[string]string {
	listenersArn := make(map[string]string)
	for _, l := range a.Listeners {
		for _, l2 := range listeners {
			if l.Protocol != nil && strings.ToLower(aws.StringValue(l.Protocol)) == strings.ToLower(l2) {
				listenersArn[strings.ToLower(aws.StringValue(l.Protocol))] = aws.StringValue(l.ListenerArn)
			}
		}
	}
	for k, v := range listenersArn {
		albLogger.Debugf("getListenersArnForProtocol: resolved %s to %s", k, v)
	}

	return listenersArn
}

/*
 * Gets listeners ARN based on http / https string
 */
func (a *ALB) GetListenerArnForProtocol(listener string) string {
	listeners := a.getListenersArnForProtocol([]string{listener})
	if val, ok := listeners[listener]; ok {
		return val
	}
	return ""
}

/*
 * modify an existing rule to a https redirect
 */
func (a *ALB) UpdateRuleToHTTPSRedirect(targetGroupArn, ruleArn string, ruleType string, rules []string) error {
	svc := elbv2.New(session.New())
	input := &elbv2.ModifyRuleInput{
		Actions: []*elbv2.Action{
			{
				RedirectConfig: &elbv2.RedirectActionConfig{
					Protocol:   aws.String("HTTPS"),
					StatusCode: aws.String("HTTP_301"),
					Port:       aws.String("443"),
				},
				Type: aws.String("redirect"),
			},
		},
		RuleArn: aws.String(ruleArn),
	}
	conditions, err := a.getRuleConditions(ruleType, rules)
	if err != nil {
		return err
	}
	input.SetConditions(conditions)

	_, err = svc.ModifyRule(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			albLogger.Errorf(aerr.Error())
		} else {
			albLogger.Errorf(err.Error())
		}
		return errors.New("Could not modify alb rule")
	}
	return nil
}

func (a *ALB) UpdateRule(targetGroupArn, ruleArn string, ruleType string, rules []string, cognitoAuth service.DeployRuleConditionsCognitoAuth) error {
	svc := elbv2.New(session.New())
	input := &elbv2.ModifyRuleInput{
		Actions: []*elbv2.Action{
			{
				TargetGroupArn: aws.String(targetGroupArn),
				Type:           aws.String("forward"),
			},
		},
		RuleArn: aws.String(ruleArn),
	}
	conditions, err := a.getRuleConditions(ruleType, rules)
	if err != nil {
		return err
	}
	input.SetConditions(conditions)

	// cognito
	if cognitoAuth.UserPoolName != "" && cognitoAuth.ClientName != "" {
		cognitoAction, err := a.getCognitoAction(targetGroupArn, cognitoAuth)
		if err != nil {
			return err
		}
		input.SetActions(cognitoAction)
	}
	_, err = svc.ModifyRule(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			albLogger.Errorf(aerr.Error())
		} else {
			albLogger.Errorf(err.Error())
		}
		return errors.New("Could not modify alb rule")
	}
	return nil
}

func (a *ALB) getRuleConditions(ruleType string, rules []string) ([]*elbv2.RuleCondition, error) {
	if ruleType == "pathPattern" {
		if len(rules) != 1 {
			return nil, errors.New("Wrong number of rules (expected 1, got " + strconv.Itoa(len(rules)) + ")")
		}
		return []*elbv2.RuleCondition{
			{
				Field:  aws.String("path-pattern"),
				Values: []*string{aws.String(rules[0])},
			},
		}, nil
	} else if ruleType == "hostname" {
		if len(rules) != 1 {
			return nil, errors.New("Wrong number of rules (expected 1, got " + strconv.Itoa(len(rules)) + ")")
		}
		hostname := rules[0]
		return []*elbv2.RuleCondition{
			{
				Field:  aws.String("host-header"),
				Values: []*string{aws.String(hostname)},
			},
		}, nil
	} else if ruleType == "combined" {
		if len(rules) != 2 {
			return nil, errors.New("Wrong number of rules (expected 2, got " + strconv.Itoa(len(rules)) + ")")
		}
		hostname := rules[1]
		return []*elbv2.RuleCondition{
			{
				Field:  aws.String("path-pattern"),
				Values: []*string{aws.String(rules[0])},
			},
			{
				Field:  aws.String("host-header"),
				Values: []*string{aws.String(hostname)},
			},
		}, nil

	} else {
		return nil, errors.New("ruleType not recognized: " + ruleType)
	}
}

func (a *ALB) CreateHTTPSRedirectRule(ruleType string, listenerArn string, targetGroupArn string, rules []string, priority int64) error {
	svc := elbv2.New(session.New())
	input := &elbv2.CreateRuleInput{
		Actions: []*elbv2.Action{
			{
				RedirectConfig: &elbv2.RedirectActionConfig{
					Protocol:   aws.String("HTTPS"),
					StatusCode: aws.String("HTTP_301"),
					Port:       aws.String("443"),
				},
				Type: aws.String("redirect"),
			},
		},
		ListenerArn: aws.String(listenerArn),
		Priority:    aws.Int64(priority),
	}
	conditions, err := a.getRuleConditions(ruleType, rules)
	if err != nil {
		return err
	}
	input.SetConditions(conditions)

	_, err = svc.CreateRule(input)
	if err != nil {
		albLogger.Errorf(err.Error())
		return fmt.Errorf("Could not create alb rule: %+v", input)
	}
	return nil
}

func (a *ALB) CreateRule(ruleType string, listenerArn string, targetGroupArn string, rules []string, priority int64, cognitoAuth service.DeployRuleConditionsCognitoAuth) error {
	svc := elbv2.New(session.New())
	input := &elbv2.CreateRuleInput{
		Actions: []*elbv2.Action{
			{
				TargetGroupArn: aws.String(targetGroupArn),
				Type:           aws.String("forward"),
			},
		},
		ListenerArn: aws.String(listenerArn),
		Priority:    aws.Int64(priority),
	}
	conditions, err := a.getRuleConditions(ruleType, rules)
	if err != nil {
		return err
	}
	input.SetConditions(conditions)

	// cognito
	if cognitoAuth.UserPoolName != "" && cognitoAuth.ClientName != "" {
		cognitoAction, err := a.getCognitoAction(targetGroupArn, cognitoAuth)
		if err != nil {
			return err
		}
		input.SetActions(cognitoAction)
	}

	_, err = svc.CreateRule(input)
	if err != nil {
		albLogger.Errorf(err.Error())
		return errors.New("Could not create alb rule")
	}
	return nil
}

// get rules by listener
func (a *ALB) GetRulesForAllListeners() error {
	a.Rules = make(map[string][]*elbv2.Rule)
	svc := elbv2.New(session.New())

	for _, l := range a.Listeners {
		input := &elbv2.DescribeRulesInput{ListenerArn: aws.String(*l.ListenerArn)}

		result, err := svc.DescribeRules(input)
		if err != nil {
			if aerr, ok := err.(awserr.Error); ok {
				switch aerr.Code() {
				case elbv2.ErrCodeListenerNotFoundException:
					albLogger.Errorf(elbv2.ErrCodeListenerNotFoundException+": %v", aerr.Error())
				case elbv2.ErrCodeRuleNotFoundException:
					albLogger.Errorf(elbv2.ErrCodeRuleNotFoundException+": %v", aerr.Error())
				default:
					albLogger.Errorf(aerr.Error())
				}
			} else {
				albLogger.Errorf(err.Error())
			}
			return errors.New("Could not get Listeners for loadbalancer")
		}
		for _, r := range result.Rules {
			a.Rules[*l.ListenerArn] = append(a.Rules[*l.ListenerArn], r)
			if len(r.Conditions) != 0 && len(r.Conditions[0].Values) != 0 {
				albLogger.Debugf("Importing rule: %+v (prio: %v)", *r.Conditions[0].Values[0], *r.Priority)
			}
		}
	}
	return nil
}
func (a *ALB) GetRulesByTargetGroupArn(targetGroupArn string) []string {
	var result []string
	for _, rules := range a.Rules {
		for _, rule := range rules {
			for _, ruleAction := range rule.Actions {
				if aws.StringValue(ruleAction.TargetGroupArn) == targetGroupArn {
					result = append(result, aws.StringValue(rule.RuleArn))
				}
			}
		}
	}
	return result
}
func (a *ALB) GetRuleByTargetGroupArnWithAuth(targetGroupArn string) []string {
	var result []string
	for _, rules := range a.Rules {
		for _, rule := range rules {
			foundAuthType := false
			for _, ruleAction := range rule.Actions {
				if aws.StringValue(ruleAction.Type) == "authenticate-cognito" {
					foundAuthType = true
				}
			}
			if foundAuthType {
				for _, ruleAction := range rule.Actions {
					if aws.StringValue(ruleAction.TargetGroupArn) == targetGroupArn {
						result = append(result, aws.StringValue(rule.RuleArn))
					}
				}
			}
		}
	}
	return result
}
func (a *ALB) GetConditionsForRule(ruleArn string) ([]string, []string) {
	conditionFields := []string{}
	conditionValues := []string{}
	for _, rules := range a.Rules {
		for _, rule := range rules {
			if aws.StringValue(rule.RuleArn) == ruleArn {
				for _, condition := range rule.Conditions {
					if aws.StringValue(condition.Field) == "path-pattern" || aws.StringValue(condition.Field) == "host-header" {
						conditionFields = append(conditionFields, aws.StringValue(condition.Field))
						if len(condition.Values) >= 1 {
							conditionValues = append(conditionValues, aws.StringValue(condition.Values[0]))
						}
					}
				}
			}
		}
	}
	return conditionFields, conditionValues
}

func (a *ALB) GetTargetGroupArn(serviceName string) (*string, error) {
	svc := elbv2.New(session.New())
	input := &elbv2.DescribeTargetGroupsInput{
		Names: []*string{aws.String(util.TruncateString(serviceName, 32))},
	}

	result, err := svc.DescribeTargetGroups(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case elbv2.ErrCodeLoadBalancerNotFoundException:
				albLogger.Errorf(elbv2.ErrCodeLoadBalancerNotFoundException+": %v", aerr.Error())
			case elbv2.ErrCodeTargetGroupNotFoundException:
				albLogger.Errorf(elbv2.ErrCodeTargetGroupNotFoundException+": %v", aerr.Error())
			default:
				albLogger.Errorf(aerr.Error())
			}
		} else {
			albLogger.Errorf(err.Error())
		}
		return nil, err
	}
	if len(result.TargetGroups) == 1 {
		return result.TargetGroups[0].TargetGroupArn, nil
	} else {
		if len(result.TargetGroups) == 0 {
			return nil, errors.New("No ALB target group found for service: " + serviceName)
		} else {
			return nil, errors.New("Multiple target groups found for service: " + serviceName + " (" + string(len(result.TargetGroups)) + ")")
		}
	}
}
func (a *ALB) GetDomain() string {
	return util.GetEnv("LOADBALANCER_DOMAIN", a.Domain)
}

/*
 * FindRule tries to find a matching rule in the Rules map
 */
func (a *ALB) FindRule(listener string, targetGroupArn string, conditionField []string, conditionValue []string) (*string, *string, error) {
	albLogger.Debugf("Find Rule: listener %s, targetGroupArn %s, conditionField %s, conditionValue %s", listener, targetGroupArn, strings.Join(conditionField, ","), strings.Join(conditionValue, ","))

	if len(conditionField) != len(conditionValue) {
		return nil, nil, errors.New("conditionField length not equal to conditionValue length")
	}
	// examine rules
	if rules, ok := a.Rules[listener]; ok {
		for _, r := range rules {
			for _, a := range r.Actions {
				if (aws.StringValue(a.Type) == "forward" && aws.StringValue(a.TargetGroupArn) == targetGroupArn) || aws.StringValue(a.Type) == "redirect" {
					// possible action match found, checking conditions
					matchingConditions := []bool{}
					for _, c := range r.Conditions {
						match := false
						for i := range conditionField {
							if aws.StringValue(c.Field) == conditionField[i] && len(c.Values) > 0 && aws.StringValue(c.Values[0]) == conditionValue[i] {
								match = true
							}
						}
						matchingConditions = append(matchingConditions, match)
					}
					if len(matchingConditions) == len(conditionField) && util.IsBoolArrayTrue(matchingConditions) {
						return r.RuleArn, r.Priority, nil
					}
				}
			}
		}
	} else {
		return nil, nil, errors.New("Listener not found in rule list")
	}
	return nil, nil, errors.New("Priority not found for rule: listener " + listener + ", targetGroupArn: " + targetGroupArn + ", Field: " + strings.Join(conditionField, ",") + ", Value: " + strings.Join(conditionValue, ","))
}

func (a *ALB) UpdateHealthCheck(targetGroupArn string, healthCheck service.DeployHealthCheck) error {
	svc := elbv2.New(session.New())
	input := &elbv2.ModifyTargetGroupInput{
		TargetGroupArn: aws.String(targetGroupArn),
	}
	if healthCheck.HealthyThreshold != 0 {
		input.SetHealthyThresholdCount(healthCheck.HealthyThreshold)
	}
	if healthCheck.UnhealthyThreshold != 0 {
		input.SetUnhealthyThresholdCount(healthCheck.UnhealthyThreshold)
	}
	if healthCheck.Path != "" {
		input.SetHealthCheckPath(healthCheck.Path)
	}
	if healthCheck.Port != "" {
		input.SetHealthCheckPort(healthCheck.Port)
	}
	if healthCheck.Protocol != "" {
		input.SetHealthCheckProtocol(healthCheck.Protocol)
	}
	if healthCheck.Interval != 0 {
		input.SetHealthCheckIntervalSeconds(healthCheck.Interval)
	}
	if healthCheck.Matcher != "" {
		input.SetMatcher(&elbv2.Matcher{HttpCode: aws.String(healthCheck.Matcher)})
	}
	if healthCheck.Timeout > 0 {
		input.SetHealthCheckTimeoutSeconds(healthCheck.Timeout)
	}
	_, err := svc.ModifyTargetGroup(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			albLogger.Errorf(aerr.Error())
			return aerr
		}
		albLogger.Errorf(err.Error())
		return err
	}
	return nil
}

func (a *ALB) ModifyTargetGroupAttributes(targetGroupArn string, d service.Deploy) error {
	svc := elbv2.New(session.New())
	input := &elbv2.ModifyTargetGroupAttributesInput{
		TargetGroupArn: aws.String(targetGroupArn),
		Attributes:     []*elbv2.TargetGroupAttribute{},
	}

	if d.DeregistrationDelay != -1 {
		delay := strconv.FormatInt(d.DeregistrationDelay, 10)
		input.Attributes = append(input.Attributes, &elbv2.TargetGroupAttribute{Key: aws.String("deregistration_delay.timeout_seconds"), Value: aws.String(delay)})
	}

	if d.Stickiness.Enabled {
		input.Attributes = append(input.Attributes, &elbv2.TargetGroupAttribute{Key: aws.String("stickiness.enabled"), Value: aws.String("true")})
		input.Attributes = append(input.Attributes, &elbv2.TargetGroupAttribute{Key: aws.String("stickiness.type"), Value: aws.String("lb_cookie")})
		if d.Stickiness.Duration != -1 {
			sd := strconv.FormatInt(d.Stickiness.Duration, 10)
			input.Attributes = append(input.Attributes, &elbv2.TargetGroupAttribute{Key: aws.String("stickiness.lb_cookie.duration_seconds"), Value: aws.String(sd)})
		}
	}

	if len(input.Attributes) == 0 {
		albLogger.Errorf("Tried to modify target group, but no attributes were passed")
		return nil
	}

	_, err := svc.ModifyTargetGroupAttributes(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			albLogger.Errorf(aerr.Error())
			return aerr
		}
		albLogger.Errorf(err.Error())
		return err
	}
	return nil
}
func (a *ALB) DeleteRule(ruleArn string) error {
	svc := elbv2.New(session.New())
	input := &elbv2.DeleteRuleInput{
		RuleArn: aws.String(ruleArn),
	}

	albLogger.Debugf("Deleting ALB Rule: %v", ruleArn)
	_, err := svc.DeleteRule(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			ecsLogger.Errorf("%v", aerr.Error())
		} else {
			ecsLogger.Errorf("%v", err.Error())
		}
		return err
	}
	return nil
}
func (a *ALB) getCognitoAction(targetGroupArn string, cognitoAuth service.DeployRuleConditionsCognitoAuth) ([]*elbv2.Action, error) {
	// get cognito user pool info
	cognito := CognitoIdp{}
	userPoolArn, userPoolClientID, userPoolDomain, err := cognito.getUserPoolInfo(cognitoAuth.UserPoolName, cognitoAuth.ClientName)
	if err != nil {
		return nil, err
	}
	return []*elbv2.Action{
		{
			AuthenticateCognitoConfig: &elbv2.AuthenticateCognitoActionConfig{
				OnUnauthenticatedRequest: aws.String("deny"),
				UserPoolArn:              aws.String(userPoolArn),
				UserPoolClientId:         aws.String(userPoolClientID),
				UserPoolDomain:           aws.String(userPoolDomain),
			},
			Type:  aws.String("authenticate-cognito"),
			Order: aws.Int64(1),
		},
		{
			TargetGroupArn: aws.String(targetGroupArn),
			Type:           aws.String("forward"),
			Order:          aws.Int64(2),
		},
	}, nil
}
