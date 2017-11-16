package main

import (
  "github.com/juju/loggo"
)

// logging
var paramstoreLogger = loggo.GetLogger("iam")

// Paramstore struct
type Paramstore struct { }

func (p *Paramstore) isEnabled() bool {
  if getEnv("PARAMSTORE_ENABLED", "no") == "yes" {
    return true
  } else {
    return false
  }
}

func (p *Paramstore) getParamstoreIAMPolicy(serviceName string) (string) {
  iam := IAM{}
  err := iam.getAccountId()
  accountId := iam.accountId
  if err != nil {
    accountId = ""
  }
  policy := `{
    "Version": "2012-10-17",
    "Statement": [
      {
        "Action": [
          "ssm:GetParameterHistory",
          "ssm:GetParameter",
          "ssm:GetParameters",
          "ssm:GetParametersByPath"
        ],
        "Resource": [
          "arn:aws:ssm:` + getEnv("AWS_REGION", "") + `:` + accountId + `:parameter/` + getEnv("PARAMSTORE_PREFIX", "") + `-` + getEnv("AWS_ACCOUNT_ENV", "") + `/` + serviceName + `/*"
        ],
        "Effect": "Allow"
      },
      {
        "Action": [
          "kms:Decrypt"
        ],
        "Resource": [
          "` + getEnv("PARAMSTORE_KMS_ARN", "") + `"
        ],
        "Effect": "Allow"
      }
    ]
  }`
  return policy
}
