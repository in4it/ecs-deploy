package ecs

import (
	"errors"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/servicediscovery"
	"github.com/juju/loggo"
)

// logging
var serviceDiscoveryLogger = loggo.GetLogger("servicediscovery")

// ECR struct
type ServiceDiscovery struct {
}

func (s *ServiceDiscovery) GetNamespaceArn(name string) (string, error) {
	var result string
	svc := servicediscovery.New(session.New())
	input := &servicediscovery.ListNamespacesInput{}
	pageNum := 0
	err := svc.ListNamespacesPages(input,
		func(page *servicediscovery.ListNamespacesOutput, lastPage bool) bool {
			pageNum++
			for _, v := range page.Namespaces {
				if aws.StringValue(v.Name) == name {
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
		return result, errors.New("Namespace not found namespace=" + name)
	}
	return result, nil
}
