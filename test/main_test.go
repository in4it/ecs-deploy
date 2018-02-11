package test

import (
	"testing"

	"github.com/in4it/ecs-deploy/provider/ecs"
	"github.com/in4it/ecs-deploy/util"
	"github.com/juju/loggo"
)

var accountId *string

const noAWSMsg = "AWS Credentials not found - test skipped"

func init() {
	// set logging to debug
	if util.GetEnv("DEBUG", "") == "true" {
		loggo.ConfigureLoggers(`<root>=DEBUG`)
	}
	// check AWS access first
	iam := ecs.IAM{}
	err := iam.GetAccountId()
	if err != nil {
		return
	}
	accountId = &iam.AccountId
}

func TestGetEnv(t *testing.T) {
	if util.GetEnv("does-not-exist", "ok") != "ok" {
		t.Errorf("env does-not-exist is not supposed to exist")
	}
}
