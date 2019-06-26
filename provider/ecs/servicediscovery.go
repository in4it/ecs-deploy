package ecs

import (
	"errors"
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/servicediscovery"
	"github.com/in4it/ecs-deploy/util"
	"github.com/juju/loggo"
)

// logging
var serviceDiscoveryLogger = loggo.GetLogger("servicediscovery")

// ECR struct
type ServiceDiscovery struct {
}

func (s *ServiceDiscovery) getNamespaceArnAndId(name string) (string, string, error) {
	var result string
	var id string
	svc := servicediscovery.New(session.New())
	input := &servicediscovery.ListNamespacesInput{}
	pageNum := 0
	err := svc.ListNamespacesPages(input,
		func(page *servicediscovery.ListNamespacesOutput, lastPage bool) bool {
			pageNum++
			for _, v := range page.Namespaces {
				if aws.StringValue(v.Name) == name {
					result = aws.StringValue(v.Arn)
					id = aws.StringValue(v.Id)
				}
			}
			return pageNum <= 100
		})

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			ecsLogger.Errorf(aerr.Error())
		} else {
			ecsLogger.Errorf(err.Error())
		}
	}
	if result == "" {
		return result, id, errors.New("Namespace not found namespace=" + name)
	}
	return result, id, nil
}
func (s *ServiceDiscovery) getServiceArn(serviceName, namespaceID string) (string, error) {
	var result string
	svc := servicediscovery.New(session.New())
	input := &servicediscovery.ListServicesInput{
		Filters: []*servicediscovery.ServiceFilter{
			{
				Name:      aws.String("NAMESPACE_ID"),
				Condition: aws.String("EQ"),
				Values:    aws.StringSlice([]string{namespaceID}),
			},
		},
	}
	pageNum := 0
	err := svc.ListServicesPages(input,
		func(page *servicediscovery.ListServicesOutput, lastPage bool) bool {
			pageNum++
			for _, v := range page.Services {
				if aws.StringValue(v.Name) == serviceName {
					result = aws.StringValue(v.Arn)
				}
			}
			return pageNum <= 100
		})

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			ecsLogger.Errorf(aerr.Error())
		} else {
			ecsLogger.Errorf(err.Error())
		}
	}
	if result == "" {
		return result, errors.New("Service not found service=" + serviceName)
	}
	return result, nil
}
func (s *ServiceDiscovery) createService(serviceName, namespaceID string) (string, error) {
	var (
		ttl              int64
		failureThreshold int64
		err              error
		output           string
	)
	ttl, err = strconv.ParseInt(util.GetEnv("SERVICE_DISCOVERY_TTL", "60"), 10, 64)
	if err != nil {
		ttl = 60
	}
	failureThreshold, err = strconv.ParseInt(util.GetEnv("SERVICE_DISCOVERY_FAILURETHRESHOLD", "1"), 10, 64)
	if err != nil {
		failureThreshold = 1
	}
	svc := servicediscovery.New(session.New())
	input := &servicediscovery.CreateServiceInput{
		CreatorRequestId: aws.String(serviceName + "-" + util.RandStringBytesMaskImprSrc(8)),
		Description:      aws.String(serviceName),
		Name:             aws.String(serviceName),
		NamespaceId:      aws.String(namespaceID),
		DnsConfig: &servicediscovery.DnsConfig{
			DnsRecords: []*servicediscovery.DnsRecord{
				{
					TTL:  aws.Int64(ttl),
					Type: aws.String("SRV"),
				},
				{
					TTL:  aws.Int64(ttl),
					Type: aws.String("A"),
				},
			},
		},
		HealthCheckCustomConfig: &servicediscovery.HealthCheckCustomConfig{
			FailureThreshold: aws.Int64(failureThreshold),
		},
	}
	result, err := svc.CreateService(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			ecsLogger.Errorf("%v", aerr.Error())
		} else {
			ecsLogger.Errorf("%v", err.Error())
		}
		return output, err
	}

	output = aws.StringValue(result.Service.Arn)

	return output, nil
}
