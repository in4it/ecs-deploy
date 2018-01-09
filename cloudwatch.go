package main

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/juju/loggo"
)

// logging
var cloudwatchLogger = loggo.GetLogger("cloudwatch")

type CloudWatch struct{}

func (cloudwatch *CloudWatch) createLogGroup(clusterName, logGroup string) error {
	svc := cloudwatchlogs.New(session.New())
	input := &cloudwatchlogs.CreateLogGroupInput{
		LogGroupName: aws.String(logGroup),
	}
	tags := make(map[string]*string)
	tags["Cluster"] = aws.String(clusterName)
	input.SetTags(tags)

	_, err := svc.CreateLogGroup(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			ecsLogger.Errorf("%v", aerr.Error())
		} else {
			ecsLogger.Errorf("%v", err.Error())
		}
		return err
	}
	return nil

}

func (cloudwatch *CloudWatch) deleteLogGroup(logGroup string) error {
	svc := cloudwatchlogs.New(session.New())
	input := &cloudwatchlogs.DeleteLogGroupInput{
		LogGroupName: aws.String(logGroup),
	}

	_, err := svc.DeleteLogGroup(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			ecsLogger.Errorf("%v", aerr.Error())
		} else {
			ecsLogger.Errorf("%v", err.Error())
		}
		return err
	}
	return nil
}
