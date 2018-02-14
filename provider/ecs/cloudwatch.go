package ecs

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/in4it/ecs-deploy/service"
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

func (cloudwatch *CloudWatch) CreateLogGroup(clusterName, logGroup string) error {
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

func (cloudwatch *CloudWatch) DeleteLogGroup(logGroup string) error {
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

func (cloudwatch *CloudWatch) GetLogEventsByTime(logGroup, logStream string, startTime, endTime time.Time, nextToken string) (CloudWatchLog, error) {
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

func (c *CloudWatch) PutMetricAlarm(serviceName, clusterName, alarmName string, alarmActions []string, alarmDescription string, datapointsToAlarm int64, metricName string, namespace string, period int64, threshold float64, comparisonOperator string, statistic string, evaluationPeriods int64) error {
	svc := cloudwatch.New(session.New())
	input := &cloudwatch.PutMetricAlarmInput{
		ActionsEnabled:     aws.Bool(true),
		AlarmActions:       aws.StringSlice(alarmActions),
		AlarmDescription:   aws.String(alarmDescription),
		AlarmName:          aws.String(alarmName),
		ComparisonOperator: aws.String(comparisonOperator),
		DatapointsToAlarm:  aws.Int64(datapointsToAlarm),
		Dimensions: []*cloudwatch.Dimension{
			{Name: aws.String("ClusterName"), Value: aws.String(clusterName)},
			{Name: aws.String("ServiceName"), Value: aws.String(serviceName)},
		},
		EvaluationPeriods: aws.Int64(evaluationPeriods),
		MetricName:        aws.String(metricName),
		Namespace:         aws.String(namespace),
		Period:            aws.Int64(period),
		Threshold:         aws.Float64(threshold),
		Statistic:         aws.String(statistic),
	}

	_, err := svc.PutMetricAlarm(input)
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

func (c *CloudWatch) DescribeAlarms(alarmNames []string) ([]service.AutoscalingPolicy, error) {
	var metricAlarms []*cloudwatch.MetricAlarm
	var aps []service.AutoscalingPolicy
	svc := cloudwatch.New(session.New())
	input := &cloudwatch.DescribeAlarmsInput{
		AlarmNames: aws.StringSlice(alarmNames),
	}

	pageNum := 0
	err := svc.DescribeAlarmsPages(input,
		func(page *cloudwatch.DescribeAlarmsOutput, lastPage bool) bool {
			pageNum++
			metricAlarms = append(metricAlarms, page.MetricAlarms...)
			return pageNum <= 100
		})

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			ecsLogger.Errorf("%v", aerr.Error())
		} else {
			ecsLogger.Errorf("%v", err.Error())
		}
		return aps, err
	}

	for _, v := range metricAlarms {
		ap := service.AutoscalingPolicy{}
		ap.PolicyName = aws.StringValue(v.AlarmName)
		ap.ComparisonOperator = aws.StringValue(v.ComparisonOperator)
		ap.Metric = aws.StringValue(v.MetricName)
		ap.Period = aws.Int64Value(v.Period)
		ap.EvaluationPeriods = aws.Int64Value(v.EvaluationPeriods)
		ap.Threshold = aws.Float64Value(v.Threshold)
		ap.ThresholdStatistic = aws.StringValue(v.Statistic)
		ap.DatapointsToAlarm = aws.Int64Value(v.DatapointsToAlarm)
		aps = append(aps, ap)
	}
	return aps, nil
}

func (c *CloudWatch) DeleteAlarms(alarmNames []string) error {
	svc := cloudwatch.New(session.New())

	input := &cloudwatch.DeleteAlarmsInput{
		AlarmNames: aws.StringSlice(alarmNames),
	}

	_, err := svc.DeleteAlarms(input)
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
