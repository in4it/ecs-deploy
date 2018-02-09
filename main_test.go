package ecsdeploy

import (
	"testing"

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
	iam := IAM{}
	err := iam.getAccountId()
	if err != nil {
		return
	}
	accountId = &iam.accountId
}

func TestGetEnv(t *testing.T) {
	if util.GetEnv("does-not-exist", "ok") != "ok" {
		t.Errorf("env does-not-exist is not supposed to exist")
	}
}
