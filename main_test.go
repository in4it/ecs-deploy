package ecsdeploy

import (
	"github.com/juju/loggo"
	"testing"
)

var accountId *string

const noAWSMsg = "AWS Credentials not found - test skipped"

func init() {
	// set logging to debug
	if getEnv("DEBUG", "") == "true" {
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
	if getEnv("does-not-exist", "ok") != "ok" {
		t.Errorf("env does-not-exist is not supposed to exist")
	}
}
