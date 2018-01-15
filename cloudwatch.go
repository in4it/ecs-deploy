package main

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/juju/loggo"

	"time"
)

type CloudWatchLog struct {
	NextBackwardToken string               `json:"nextBackwardToken"`
	NextForwardToken  string               `json:"nextForwardToken"`
	LogEvents         []CloudWatchLogEvent `json:"logEvents"`
}
type CloudWatchLogEvent struct {
	IngestionTime time.Time `json:"ingestionTime"`
	Message       string    `json:"message"`
	Timestamp     time.Time `json:"timestamp"`
}

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

func (cloudwatch *CloudWatch) getLogEventsByTime(logGroup, logStream string, startTime, endTime time.Time, nextToken string) (CloudWatchLog, error) {
	var logEvents CloudWatchLog
	svc := cloudwatchlogs.New(session.New())
	input := &cloudwatchlogs.GetLogEventsInput{
		LogGroupName:  aws.String(logGroup),
		LogStreamName: aws.String(logStream),
		StartTime:     aws.Int64(startTime.UnixNano() / 1000000),
		EndTime:       aws.Int64(endTime.UnixNano() / 1000000),
	}
	if nextToken != "" {
		input.SetNextToken(nextToken)
	}

	result, err := svc.GetLogEvents(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			ecsLogger.Errorf("%v", aerr.Error())
		} else {
			ecsLogger.Errorf("%v", err.Error())
		}
		return logEvents, err
	}
	logEvents.NextBackwardToken = aws.StringValue(result.NextBackwardToken)
	logEvents.NextForwardToken = aws.StringValue(result.NextForwardToken)
	for _, v := range result.Events {
		var l CloudWatchLogEvent
		l.IngestionTime = time.Unix(0, aws.Int64Value(v.IngestionTime)*1000000)
		l.Timestamp = time.Unix(0, aws.Int64Value(v.Timestamp)*1000000)
		l.Message = aws.StringValue(v.Message)
		logEvents.LogEvents = append(logEvents.LogEvents, l)
	}
	return logEvents, nil
}
