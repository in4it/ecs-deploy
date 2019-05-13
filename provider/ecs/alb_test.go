package ecs

import (
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/in4it/ecs-deploy/util"
)

func TestGetHighestRule(t *testing.T) {
	if accountId == nil {
		t.Skip(noAWSMsg)
	}
	a, err := NewALB(util.GetEnv("TEST_CLUSTERNAME", "test-cluster"))
	if err != nil {
		t.Errorf("Error: %v", err)
		return
	}
	highest, err := a.GetHighestRule()
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	fmt.Printf("Highest rule in ALB (%v) is: %d ", a.loadBalancerName, highest)
}

func TestFindRule(t *testing.T) {
	a := ALB{}
	a.Rules = make(map[string][]*elbv2.Rule)
	a.Rules["listener"] = []*elbv2.Rule{
		{
			RuleArn:  aws.String("1"),
			Priority: aws.String("1"),
			Actions: []*elbv2.Action{
				{
					Type:           aws.String("forward"),
					TargetGroupArn: aws.String("targetGroup"),
				},
			},
			Conditions: []*elbv2.RuleCondition{
				{
					Field:  aws.String("host-header"),
					Values: []*string{aws.String("host.example.com")},
				},
			},
		},
		{
			RuleArn:  aws.String("2"),
			Priority: aws.String("2"),
			Actions: []*elbv2.Action{
				{
					Type:           aws.String("forward"),
					TargetGroupArn: aws.String("targetGroup"),
				},
			},
			Conditions: []*elbv2.RuleCondition{
				{
					Field:  aws.String("host-header"),
					Values: []*string{aws.String("host-2.example.com")},
				},
				{
					Field:  aws.String("path-pattern"),
					Values: []*string{aws.String("/api")},
				},
			},
		},
		{
			RuleArn:  aws.String("3"),
			Priority: aws.String("3"),
			Actions: []*elbv2.Action{
				{
					Type:           aws.String("forward"),
					TargetGroupArn: aws.String("targetGroup"),
				},
			},
			Conditions: []*elbv2.RuleCondition{
				{
					Field:  aws.String("host-header"),
					Values: []*string{aws.String("host.example.com")},
				},
				{
					Field:  aws.String("path-pattern"),
					Values: []*string{aws.String("/api/v1")},
				},
			},
		},
		{
			RuleArn:  aws.String("4"),
			Priority: aws.String("4"),
			Actions: []*elbv2.Action{
				{
					Type:           aws.String("forward"),
					TargetGroupArn: aws.String("targetGroup"),
				},
			},
			Conditions: []*elbv2.RuleCondition{
				{
					Field:  aws.String("host-header"),
					Values: []*string{aws.String("host.example.com")},
				},
				{
					Field:  aws.String("path-pattern"),
					Values: []*string{aws.String("/api")},
				},
			},
		},
	}
	conditionField := []string{"host-header", "path-pattern"}
	conditionValue := []string{"host.example.com", "/api"}
	ruleArn, priority, err := a.FindRule("listener", "targetGroup", conditionField, conditionValue)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	if *priority != "4" || *ruleArn != "4" {
		t.Errorf("Error: found wrong rule")
	}
	// re-order
	a.Rules["listener"][0], a.Rules["listener"][3] = a.Rules["listener"][3], a.Rules["listener"][0]
	ruleArn, priority, err = a.FindRule("listener", "targetGroup", conditionField, conditionValue)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	if *priority != "4" || *ruleArn != "4" {
		t.Errorf("Error: found wrong rule")
	}
}

func TestGetListenersArnForProtocol(t *testing.T) {
	a := ALB{}
	a.Listeners = []*elbv2.Listener{
		{
			Protocol:    aws.String("HTTP"),
			ListenerArn: aws.String("arn:aws:elasticloadbalancing:region:1234567890:listener/app/myapp/abc123"),
		},
		{
			Protocol:    aws.String("HTTPS"),
			ListenerArn: aws.String("arn:aws:elasticloadbalancing:region:1234567890:listener/app/myapp/def456"),
		},
	}
	listeners := []string{"http", "https"}
	retListeners := a.getListenersArnForProtocol(listeners)
	expectedResult := map[string]string{
		"http":  "arn:aws:elasticloadbalancing:region:1234567890:listener/app/myapp/abc123",
		"https": "arn:aws:elasticloadbalancing:region:1234567890:listener/app/myapp/def456",
	}
	if retListeners["http"] != expectedResult["http"] {
		t.Errorf("didn't get expected result: got %s, expected %s", retListeners, expectedResult)
	}
	if retListeners["https"] != expectedResult["https"] {
		t.Errorf("didn't get expected result: got %s, expected %s", retListeners, expectedResult)
	}
}

func TestGetListenerArnForProtocol(t *testing.T) {
	a := ALB{}
	a.Listeners = []*elbv2.Listener{
		{
			Protocol:    aws.String("HTTP"),
			ListenerArn: aws.String("arn:aws:elasticloadbalancing:region:1234567890:listener/app/myapp/abc123"),
		},
		{
			Protocol:    aws.String("HTTPS"),
			ListenerArn: aws.String("arn:aws:elasticloadbalancing:region:1234567890:listener/app/myapp/def456"),
		},
	}
	retListener := a.GetListenerArnForProtocol("http")
	expectedResult := aws.StringValue(a.Listeners[0].ListenerArn)
	if retListener != expectedResult {
		t.Errorf("didn't get expected result: got %s, expected %s", retListener, expectedResult)
	}
}
