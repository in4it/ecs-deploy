package ecs

import (
	"errors"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/in4it/ecs-deploy/service"
	"github.com/in4it/ecs-deploy/util"
	"github.com/juju/loggo"
)

// logging
var paramstoreLogger = loggo.GetLogger("paramstore")

// parameter type
type Parameter struct {
	Arn     string `json:"arn"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Value   string `json:"value"`
	Version int64  `json:"version"`
}

// Paramstore struct
type Paramstore struct {
	Parameters      map[string]Parameter
	SsmAssumingRole *ssm.SSM
}

func (p *Paramstore) IsEnabled() bool {
	if util.GetEnv("PARAMSTORE_ENABLED", "no") == "yes" {
		return true
	} else {
		return false
	}
}

func (p *Paramstore) GetPrefix() string {
	if util.GetEnv("AWS_ENV_PATH", "") != "" {
		return util.GetEnv("AWS_ENV_PATH", "")
	} else {
		if util.GetEnv("PARAMSTORE_PREFIX", "") == "" {
			return ""
		} else {
			return "/" + util.GetEnv("PARAMSTORE_PREFIX", "") + "-" + util.GetEnv("AWS_ACCOUNT_ENV", "") + "/ecs-deploy/"
		}
	}
}
func (p *Paramstore) GetPrefixForService(serviceName string) string {
	if util.GetEnv("PARAMSTORE_PREFIX", "") == "" {
		return ""
	} else {
		return "/" + util.GetEnv("PARAMSTORE_PREFIX", "") + "-" + util.GetEnv("AWS_ACCOUNT_ENV", "") + "/" + serviceName + "/"
	}
}
func (p *Paramstore) AssumeRole(roleArn, roleSessionName, prevCreds string) (string, error) {
	iam := IAM{}
	creds, jsonCreds, err := iam.AssumeRole(roleArn, roleSessionName, prevCreds)
	if err != nil {
		return "", err
	}
	// assume role
	sess := session.Must(session.NewSession())
	p.SsmAssumingRole = ssm.New(sess, &aws.Config{Credentials: creds})
	if p.SsmAssumingRole == nil {
		return "", errors.New("Could not assume role")
	}
	paramstoreLogger.Debugf("Assumed role %v with roleSessionName %v", roleArn, roleSessionName)

	return jsonCreds, nil
}
func (p *Paramstore) GetParameters(prefix string, withDecryption bool) error {
	var svc *ssm.SSM
	p.Parameters = make(map[string]Parameter)
	if prefix == "" {
		// no valid prefix - parameter store not in use
		return nil
	}
	if p.SsmAssumingRole == nil {
		svc = ssm.New(session.New())
	} else {
		svc = p.SsmAssumingRole
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
				p.Parameters[paramName] = Parameter{
					Arn:     aws.StringValue(param.ARN),
					Name:    aws.StringValue(param.Name),
					Type:    aws.StringValue(param.Type),
					Value:   value,
					Version: aws.Int64Value(param.Version),
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
func (p *Paramstore) GetParameterValue(name string) (*string, error) {
	if param, ok := p.Parameters[name]; ok {
		if param.Value != "" {
			return &param.Value, nil
		}
	} else {
		return nil, errors.New("Tried getParameterValue on parameter that doesn't exist: " + name)
	}

	// val not found, but does exist, retrieve
	svc := ssm.New(session.New())
	input := &ssm.GetParameterInput{
		Name:           aws.String(p.GetPrefix() + name),
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

func (p *Paramstore) GetParamstoreIAMPolicy(path string) string {
	iam := IAM{}
	err := iam.GetAccountId()
	accountId := iam.AccountId
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
          "arn:aws:ssm:` + util.GetEnv("AWS_REGION", "") + `:` + accountId + `:parameter/` + util.GetEnv("PARAMSTORE_PREFIX", "") + `-` + util.GetEnv("AWS_ACCOUNT_ENV", "") + `/` + path + `/*"
        ],
        "Effect": "Allow"
      },
      {
        "Action": [
          "kms:Decrypt"
        ],
        "Resource": [
          "` + util.GetEnv("PARAMSTORE_KMS_ARN", "") + `"
        ],
        "Effect": "Allow"
      }
    ]
  }`
	return policy
}
func (p *Paramstore) PutParameter(serviceName string, parameter service.DeployServiceParameter) (*int64, error) {
	var svc *ssm.SSM
	if p.SsmAssumingRole == nil {
		svc = ssm.New(session.New())
	} else {
		svc = p.SsmAssumingRole
	}

	input := &ssm.PutParameterInput{
		Name:      aws.String(p.GetPrefixForService(serviceName) + parameter.Name),
		Value:     aws.String(parameter.Value),
		Overwrite: aws.Bool(true),
	}
	if parameter.Encrypted {
		input.SetType("SecureString")
		input.SetKeyId(util.GetEnv("PARAMSTORE_KMS_ARN", ""))
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
func (p *Paramstore) DeleteParameter(serviceName, parameter string) error {
	var svc *ssm.SSM
	if p.SsmAssumingRole == nil {
		svc = ssm.New(session.New())
	} else {
		svc = p.SsmAssumingRole
	}

	input := &ssm.DeleteParameterInput{
		Name: aws.String(p.GetPrefixForService(serviceName) + parameter),
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

func (p *Paramstore) Bootstrap(serviceName, prefix string, environment string, parameters []service.DeployServiceParameter) error {
	os.Setenv("PARAMSTORE_PREFIX", prefix)
	os.Setenv("AWS_ACCOUNT_ENV", environment)
	for _, v := range parameters {
		p.PutParameter(serviceName, v)
	}
	return nil
}

func (p *Paramstore) RetrieveKeys() error {
	if p.IsEnabled() {
		err := p.GetParameters(p.GetPrefix(), true)
		if err != nil {
			return err
		}
		for k, v := range p.Parameters {
			os.Setenv(k, v.Value)
		}
	}
	return nil
}
