package main

import (
	"errors"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/juju/loggo"
	"strings"
)

// logging
var paramstoreLogger = loggo.GetLogger("paramstore")

// Paramstore struct
type Paramstore struct {
	parameters map[string]string
}

func (p *Paramstore) isEnabled() bool {
	if getEnv("PARAMSTORE_ENABLED", "no") == "yes" {
		return true
	} else {
		return false
	}
}

func (p *Paramstore) getPrefix() string {
	if getEnv("PARAMSTORE_PREFIX", "") == "" {
		return ""
	} else {
		return "/" + getEnv("PARAMSTORE_PREFIX", "") + "-" + getEnv("AWS_ACCOUNT_ENV", "") + "/ecs-deploy/"
	}
}
func (p *Paramstore) getParameters() error {
	p.parameters = make(map[string]string)
	if p.getPrefix() == "" {
		// no valid prefix - parameter store not in use
		return nil
	}
	svc := ssm.New(session.New())
	input := &ssm.GetParametersByPathInput{
		Path:           aws.String(p.getPrefix()),
		WithDecryption: aws.Bool(true),
	}

	pageNum := 0
	err := svc.GetParametersByPathPages(input,
		func(page *ssm.GetParametersByPathOutput, lastPage bool) bool {
			pageNum++
			for _, param := range page.Parameters {
				paramName := strings.Replace(*param.Name, p.getPrefix(), "", -1)
				paramstoreLogger.Debugf("Imported parameter: %v", paramName)
				p.parameters[paramName] = *param.Value
			}
			return pageNum <= 50 // 50 iterations max
		})

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			paramstoreLogger.Errorf(aerr.Error())
		} else {
			paramstoreLogger.Errorf(err.Error())
		}
		return err
	}
	return nil
}
func (p *Paramstore) getParameterValue(name string) (*string, error) {
	if val, ok := p.parameters[name]; ok {
		if val != "" {
			return &val, nil
		}
	} else {
		return nil, errors.New("Tried getParameterValue on parameter that doesn't exist: " + name)
	}

	// val not found, but does exist, retrieve
	svc := ssm.New(session.New())
	input := &ssm.GetParameterInput{
		Name:           aws.String(p.getPrefix() + name),
		WithDecryption: aws.Bool(true),
	}

	result, err := svc.GetParameter(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case ssm.ErrCodeParameterNotFound:
				paramstoreLogger.Errorf("%v: %v", ssm.ErrCodeParameterNotFound, aerr.Error())
				return nil, errors.New("ParameterNotFound")
			default:
				paramstoreLogger.Errorf(aerr.Error())
			}
		} else {
			paramstoreLogger.Errorf(err.Error())
		}
		return nil, err
	}
	return result.Parameter.Value, nil
}

func (p *Paramstore) getParamstoreIAMPolicy(serviceName string) string {
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
