package ecs

import (
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
	err := iam.GetAccountId()
	if err != nil {
		return
	}
	accountId = &iam.AccountId
}
