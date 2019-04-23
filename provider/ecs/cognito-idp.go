package ecs

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cognitoidentityprovider"
	"github.com/juju/loggo"
)

// logging
var cognitoLogger = loggo.GetLogger("cognito")

// Cognito struct
type CognitoIdp struct {
}

func (c *CognitoIdp) getUserPoolInfo(userPoolName, userPoolClientName string) (string, string, string, error) {
	userPoolID, err := c.getUserPoolArn(userPoolName)
	if err != nil {
		return "", "", "", err
	}

	userPool, err := c.describeUserPool(userPoolID)
	if err != nil {
		return "", "", "", err
	}

	userPoolClientID, err := c.getUserPoolClientID(userPoolID, userPoolClientName)
	if err != nil {
		return "", "", "", err
	}

	return aws.StringValue(userPool.Arn), userPoolClientID, aws.StringValue(userPool.Domain), nil
}

func (c *CognitoIdp) describeUserPool(userPoolID string) (*cognitoidentityprovider.UserPoolType, error) {
	svc := cognitoidentityprovider.New(session.New())
	input := &cognitoidentityprovider.DescribeUserPoolInput{
		UserPoolId: aws.String(userPoolID),
	}

	res, err := svc.DescribeUserPool(input)
	if err != nil {
		return nil, err
	}

	return res.UserPool, nil
}

func (c *CognitoIdp) getUserPoolArn(userPoolName string) (string, error) {
	svc := cognitoidentityprovider.New(session.New())
	input := &cognitoidentityprovider.ListUserPoolsInput{
		MaxResults: aws.Int64(60),
	}

	userPoolID := ""

	pageNum := 0
	err := svc.ListUserPoolsPages(input,
		func(page *cognitoidentityprovider.ListUserPoolsOutput, lastPage bool) bool {
			pageNum++
			for _, userPool := range page.UserPools {
				if aws.StringValue(userPool.Name) == userPoolName {
					userPoolID = aws.StringValue(userPool.Id)
				}
			}
			return pageNum <= 100
		})

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			cognitoLogger.Errorf(aerr.Error())
		} else {
			cognitoLogger.Errorf(err.Error())
		}
		return userPoolID, err
	}
	if userPoolID == "" {
		return userPoolID, fmt.Errorf("Could not find userpool with name %s", userPoolName)
	}
	return userPoolID, nil
}
func (c *CognitoIdp) getUserPoolClientID(userPoolID, userPoolClientName string) (string, error) {
	svc := cognitoidentityprovider.New(session.New())
	input := &cognitoidentityprovider.ListUserPoolClientsInput{
		UserPoolId: aws.String(userPoolID),
	}

	userPoolClientNameID := ""

	pageNum := 0
	err := svc.ListUserPoolClientsPages(input,
		func(page *cognitoidentityprovider.ListUserPoolClientsOutput, lastPage bool) bool {
			pageNum++
			for _, userPoolClient := range page.UserPoolClients {
				if aws.StringValue(userPoolClient.ClientName) == userPoolClientName {
					userPoolClientNameID = aws.StringValue(userPoolClient.ClientId)
				}
			}
			return pageNum <= 100
		})

	if err != nil {
		return userPoolClientNameID, err
	}
	if userPoolClientNameID == "" {
		return userPoolClientNameID, fmt.Errorf("Could not find userpool client with name %s", userPoolClientName)
	}
	return userPoolClientNameID, nil
}
