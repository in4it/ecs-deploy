package ecs

import (
	"encoding/json"
	"os"
	"testing"
)

func TestGetPrefix(t *testing.T) {
	p := Paramstore{}
	os.Setenv("AWS_ENV_PATH", "/mycompany-staging/ecs-deploy/")
	os.Setenv("PARAMSTORE_PREFIX", "mycompany2")
	os.Setenv("AWS_ACCOUNT_ENV", "prod")
	if p.GetPrefix() != "/mycompany-staging/ecs-deploy/" {
		t.Errorf("Wrong prefix returned: %v", p.GetPrefix())
	}
	os.Setenv("AWS_ENV_PATH", "")
	if p.GetPrefix() != "/mycompany2-prod/ecs-deploy/" {
		t.Errorf("Wrong prefix returned: %v", p.GetPrefix())
	}
}

func TestGetParamstoreIAMPolicy(t *testing.T) {
	type IAMPolicy struct {
		Version   string `json:"Version"`
		Statement []struct {
			Action   []string `json:"Action"`
			Resource []string `json:"Resource"`
			Effect   string   `json:"Effect"`
		} `json:"Statement"`
	}
	p := Paramstore{}
	os.Setenv("AWS_ENV_PATH", "/mycluster-staging/ecs-deploy/")
	os.Setenv("PARAMSTORE_PREFIX", "mycompany2")
	os.Setenv("AWS_ACCOUNT_ENV", "staging")
	os.Setenv("AWS_REGION", "us-east-1")
	out := p.GetParamstoreIAMPolicy("myservice")
	var iamPolicy IAMPolicy
	err := json.Unmarshal([]byte(out), &iamPolicy)
	if err != nil {
		t.Errorf("unmarshal error: %s", err)
	}
	if iamPolicy.Statement[0].Resource[0] != "arn:aws:ssm:us-east-1::parameter/mycompany2-staging/myservice/*" {
		t.Errorf("unexpected resource: %s", iamPolicy.Statement[0].Resource[0])
	}
}

func TestGetParamstoreIAMPolicyWithKMS(t *testing.T) {
	type IAMPolicy struct {
		Version   string `json:"Version"`
		Statement []struct {
			Action   []string `json:"Action"`
			Resource []string `json:"Resource"`
			Effect   string   `json:"Effect"`
		} `json:"Statement"`
	}
	p := Paramstore{}
	os.Setenv("AWS_ENV_PATH", "/mycluster-staging/ecs-deploy/")
	os.Setenv("PARAMSTORE_PREFIX", "mycompany2")
	os.Setenv("AWS_ACCOUNT_ENV", "staging")
	os.Setenv("AWS_REGION", "us-east-1")
	os.Setenv("PARAMSTORE_KMS_ARN", "arn:aws:kms:us-east-1:123456:testarn")
	out := p.GetParamstoreIAMPolicy("myservice")
	var iamPolicy IAMPolicy
	err := json.Unmarshal([]byte(out), &iamPolicy)
	if err != nil {
		t.Errorf("unmarshal error: %s", err)
	}
	if iamPolicy.Statement[0].Resource[0] != "arn:aws:ssm:us-east-1::parameter/mycompany2-staging/myservice/*" {
		t.Errorf("unexpected resource: %s", iamPolicy.Statement[0].Resource[0])
	}
	if iamPolicy.Statement[1].Resource[0] != "arn:aws:kms:us-east-1:123456:testarn" {
		t.Errorf("unexpected resource: %s", iamPolicy.Statement[1].Resource[0])
	}
}
