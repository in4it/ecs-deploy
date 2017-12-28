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

// parameter type
type Parameter struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Value   string `json:"value"`
	Version int64  `json:"version"`
}

// Paramstore struct
type Paramstore struct {
	parameters      map[string]Parameter
	ssmAssumingRole *ssm.SSM
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
func (p *Paramstore) getPrefixForService(serviceName string) string {
	if getEnv("PARAMSTORE_PREFIX", "") == "" {
		return ""
	} else {
		return "/" + getEnv("PARAMSTORE_PREFIX", "") + "-" + getEnv("AWS_ACCOUNT_ENV", "") + "/" + serviceName + "/"
	}
}
func (p *Paramstore) assumeRole(roleArn, roleSessionName, prevCreds string) (string, error) {
	iam := IAM{}
	creds, jsonCreds, err := iam.assumeRole(roleArn, roleSessionName, prevCreds)
	if err != nil {
		return "", err
	}
	// assume role
	sess := session.Must(session.NewSession())
	p.ssmAssumingRole = ssm.New(sess, &aws.Config{Credentials: creds})
	if p.ssmAssumingRole == nil {
		return "", errors.New("Could not assume role")
	}
	paramstoreLogger.Debugf("Assumed role %v with roleSessionName %v", roleArn, roleSessionName)

	return jsonCreds, nil
}
func (p *Paramstore) getParameters(prefix string, withDecryption bool) error {
	var svc *ssm.SSM
	p.parameters = make(map[string]Parameter)
	if prefix == "" {
		// no valid prefix - parameter store not in use
		return nil
	}
	if p.ssmAssumingRole == nil {
		svc = ssm.New(session.New())
	} else {
		svc = p.ssmAssumingRole
	}
	input := &ssm.GetParametersByPathInput{
		Path:           aws.String(prefix),
		WithDecryption: aws.Bool(withDecryption),
	}

	pageNum := 0
	err := svc.GetParametersByPathPages(input,
		func(page *ssm.GetParametersByPathOutput, lastPage bool) bool {
			pageNum++
			for _, param := range page.Parameters {
				var value string
				paramName := strings.Replace(*param.Name, prefix, "", -1)
				paramstoreLogger.Debugf("Read parameter: %v", paramName)
				if withDecryption || *param.Type != "SecureString" {
					value = *param.Value
				} else {
					value = "***"
				}
				p.parameters[paramName] = Parameter{
					Name:    *param.Name,
					Type:    *param.Type,
					Value:   value,
					Version: *param.Version,
				}
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
	if param, ok := p.parameters[name]; ok {
		if param.Value != "" {
			return &param.Value, nil
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
func (p *Paramstore) putParameter(serviceName string, parameter DeployServiceParameter) (*int64, error) {
	var svc *ssm.SSM
	if p.ssmAssumingRole == nil {
		svc = ssm.New(session.New())
	} else {
		svc = p.ssmAssumingRole
	}

	input := &ssm.PutParameterInput{
		Name:      aws.String(p.getPrefixForService(serviceName) + parameter.Name),
		Value:     aws.String(parameter.Value),
		Overwrite: aws.Bool(true),
	}
	if parameter.Encrypted {
		input.SetType("SecureString")
		input.SetKeyId(getEnv("PARAMSTORE_KMS_ARN", ""))
	} else {
		input.SetType("String")
	}

	result, err := svc.PutParameter(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			paramstoreLogger.Errorf(aerr.Error())
		} else {
			paramstoreLogger.Errorf(err.Error())
		}
		return nil, err
	}
	return result.Version, nil
}
func (p *Paramstore) deleteParameter(serviceName, parameter string) error {
	var svc *ssm.SSM
	if p.ssmAssumingRole == nil {
		svc = ssm.New(session.New())
	} else {
		svc = p.ssmAssumingRole
	}

	input := &ssm.DeleteParameterInput{
		Name: aws.String(p.getPrefixForService(serviceName) + parameter),
	}

	_, err := svc.DeleteParameter(input)
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
