package main

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/juju/loggo"

	"errors"
	"math"
)

// logging
var ecsLogger = loggo.GetLogger("ecs")

// ECS struct
type ECS struct {
	clusterName    string
	serviceName    string
	iamRoleArn     string
	taskDefinition *ecs.RegisterTaskDefinitionInput
	taskDefArn     *string
	targetGroupArn *string
}

// Creates ECS repository
func (e *ECS) createTaskDefinition(d Deploy) (*string, error) {
	svc := ecs.New(session.New())
	e.taskDefinition = &ecs.RegisterTaskDefinitionInput{
		Family:      aws.String(e.serviceName),
		TaskRoleArn: aws.String(e.iamRoleArn),
	}

	// loop over containers
	for _, container := range d.Containers {

		// get account id
		iam := IAM{}
		err := iam.getAccountId()
		if err != nil {
			return nil, errors.New("Could not get accountId during createTaskDefinition")
		}

		// prepare image Uri
		var imageUri string
		if container.ContainerURI == "" {
			if container.ContainerImage == "" {
				imageUri = iam.accountId + ".dkr.ecr." + getEnv("AWS_REGION", "") + ".amazonaws.com" + "/" + container.ContainerName
			} else {
				imageUri = iam.accountId + ".dkr.ecr." + getEnv("AWS_REGION", "") + ".amazonaws.com" + "/" + container.ContainerImage
			}
			if container.ContainerTag != "" {
				imageUri += ":" + container.ContainerTag
			}
		} else {
			imageUri = container.ContainerURI
		}

		// prepare container definition
		containerDefinition := &ecs.ContainerDefinition{
			Name:  aws.String(container.ContainerName),
			Image: aws.String(imageUri),
		}
		// set containerPort if not empty
		if container.ContainerPort > 0 {
			containerDefinition.SetPortMappings([]*ecs.PortMapping{
				{
					ContainerPort: aws.Int64(container.ContainerPort),
				},
			})
		}
		// set containerCommand if not empty
		if len(container.ContainerCommand) > 0 {
			containerDefinition.SetCommand(container.ContainerCommand)
		}
		// set cloudwacht logs if enabled
		if getEnv("CLOUDWATCH_LOGS_ENABLED", "no") == "yes" {
			var logPrefix string
			if getEnv("CLOUDWATCH_LOGS_PREFIX", "") != "" {
				logPrefix = getEnv("CLOUDWATCH_LOGS_PREFIX", "") + "-" + getEnv("AWS_ACCOUNT_ENV", "")
			}
			containerDefinition.SetLogConfiguration(&ecs.LogConfiguration{
				LogDriver: aws.String("awslogs"),
				Options: map[string]*string{
					"awslogs-group":         aws.String(logPrefix),
					"awslogs-region":        aws.String(getEnv("AWS_REGION", "")),
					"awslogs-stream-prefix": aws.String(container.ContainerName),
				},
			})
		}
		if container.Memory > 0 {
			containerDefinition.Memory = aws.Int64(container.Memory)
		}
		if container.MemoryReservation > 0 {
			containerDefinition.MemoryReservation = aws.Int64(container.MemoryReservation)
		}

		if container.Essential {
			containerDefinition.Essential = aws.Bool(container.Essential)
		}

		if getEnv("PARAMSTORE_ENABLED", "no") == "yes" {
			containerDefinition.SetEnvironment([]*ecs.KeyValuePair{
				{Name: aws.String("AWS_REGION"), Value: aws.String(getEnv("AWS_REGION", ""))},
				{Name: aws.String("AWS_ENV_PATH"), Value: aws.String("/" + getEnv("PARAMSTORE_PREFIX", "") + "-" + getEnv("AWS_ACCOUNT_ENV", "") + "/" + e.serviceName + "/")},
			})
		}

		e.taskDefinition.ContainerDefinitions = append(e.taskDefinition.ContainerDefinitions, containerDefinition)
	}

	// going to register
	ecsLogger.Debugf("Going to register: %+v", e.taskDefinition)

	result, err := svc.RegisterTaskDefinition(e.taskDefinition)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case ecs.ErrCodeServerException:
				ecsLogger.Errorf(ecs.ErrCodeServerException+": %v", aerr.Error())
			case ecs.ErrCodeClientException:
				ecsLogger.Errorf(ecs.ErrCodeClientException+": %v", aerr.Error())
			case ecs.ErrCodeInvalidParameterException:
				ecsLogger.Errorf(ecs.ErrCodeInvalidParameterException+": %v", aerr.Error())
			default:
				ecsLogger.Errorf(aerr.Error())
			}
		}
		// return error
		return nil, errors.New("Could not register task definition")
	} else {
		return result.TaskDefinition.TaskDefinitionArn, nil
	}
}

// check whether service exists
func (e *ECS) serviceExists(serviceName string) (bool, error) {
	svc := ecs.New(session.New())
	input := &ecs.DescribeServicesInput{
		Cluster: aws.String(e.clusterName),
		Services: []*string{
			aws.String(serviceName),
		},
	}

	result, err := svc.DescribeServices(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case ecs.ErrCodeServerException:
				ecsLogger.Errorf(ecs.ErrCodeServerException, aerr.Error())
			case ecs.ErrCodeClientException:
				ecsLogger.Errorf(ecs.ErrCodeClientException, aerr.Error())
			case ecs.ErrCodeInvalidParameterException:
				ecsLogger.Errorf(ecs.ErrCodeInvalidParameterException, aerr.Error())
			case ecs.ErrCodeClusterNotFoundException:
				ecsLogger.Errorf(ecs.ErrCodeClusterNotFoundException, aerr.Error())
			default:
				ecsLogger.Errorf(aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			ecsLogger.Errorf(err.Error())
		}
		return false, err
	}
	if len(result.Services) == 0 {
		return false, nil
	} else if len(result.Services) == 1 && *result.Services[0].Status == "INACTIVE" {
		return false, nil
	} else {
		return true, nil
	}
}

// Update ECS service
func (e *ECS) updateService(serviceName string, taskDefArn *string) (*string, error) {
	svc := ecs.New(session.New())
	input := &ecs.UpdateServiceInput{
		Cluster:        aws.String(e.clusterName),
		Service:        aws.String(serviceName),
		TaskDefinition: aws.String(*taskDefArn),
	}

	result, err := svc.UpdateService(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case ecs.ErrCodeServerException:
				ecsLogger.Errorf(ecs.ErrCodeServerException+": %v", aerr.Error())
			case ecs.ErrCodeClientException:
				ecsLogger.Errorf(ecs.ErrCodeClientException+": %v", aerr.Error())
			case ecs.ErrCodeInvalidParameterException:
				ecsLogger.Errorf(ecs.ErrCodeInvalidParameterException+": %v", aerr.Error())
			case ecs.ErrCodeClusterNotFoundException:
				ecsLogger.Errorf(ecs.ErrCodeClusterNotFoundException+": %v", aerr.Error())
			case ecs.ErrCodeServiceNotFoundException:
				ecsLogger.Infof(ecs.ErrCodeServiceNotFoundException+": %v", aerr.Error())
				// return error code to create new service
				return nil, errors.New("ServiceNotFoundException")
			case ecs.ErrCodeServiceNotActiveException:
				ecsLogger.Errorf(ecs.ErrCodeServiceNotActiveException+": %v", aerr.Error())
			default:
				ecsLogger.Errorf(aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			ecsLogger.Errorf(err.Error())
		}
		return nil, errors.New("Could not update service: " + serviceName)
	}
	return result.Service.ServiceName, nil
}

// create service
func (e *ECS) createService(d Deploy) error {
	svc := ecs.New(session.New())

	// sanity checks
	if len(d.Containers) == 0 {
		return errors.New("No containers defined")
	}

	input := &ecs.CreateServiceInput{
		Cluster:      aws.String(d.Cluster),
		DesiredCount: aws.Int64(d.DesiredCount),
		LoadBalancers: []*ecs.LoadBalancer{
			{
				ContainerName:  aws.String(e.serviceName),
				ContainerPort:  aws.Int64(d.ServicePort),
				TargetGroupArn: aws.String(*e.targetGroupArn),
			},
		},
		Role:           aws.String(getEnv("AWS_ECS_SERVICE_ROLE", "ecs-service-role")),
		ServiceName:    aws.String(e.serviceName),
		TaskDefinition: aws.String(*e.taskDefArn),
	}

	// check whether min/max is set
	dc := &ecs.DeploymentConfiguration{}
	if d.MinimumHealthyPercent > 0 {
		dc.SetMinimumHealthyPercent(d.MinimumHealthyPercent)
	}
	if d.MaximumPercent > 0 {
		dc.SetMaximumPercent(d.MaximumPercent)
	}
	if (ecs.DeploymentConfiguration{}) != *dc {
		input.SetDeploymentConfiguration(dc)
	}

	// create service
	_, err := svc.CreateService(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case ecs.ErrCodeServerException:
				ecsLogger.Errorf(ecs.ErrCodeServerException+": %v", aerr.Error())
			case ecs.ErrCodeClientException:
				ecsLogger.Errorf(ecs.ErrCodeClientException+": %v", aerr.Error())
			case ecs.ErrCodeInvalidParameterException:
				ecsLogger.Errorf(ecs.ErrCodeInvalidParameterException+": %v", aerr.Error())
			case ecs.ErrCodeClusterNotFoundException:
				ecsLogger.Errorf(ecs.ErrCodeClusterNotFoundException+": %v", aerr.Error())
			default:
				ecsLogger.Errorf(aerr.Error())
			}
		} else {
			ecsLogger.Errorf(err.Error())
		}
		return errors.New("Could not create service")
	}
	return nil
}

// wait until service is stable
func (e *ECS) waitUntilServicesStable(serviceName string) error {
	svc := ecs.New(session.New())
	input := &ecs.DescribeServicesInput{
		Cluster:  aws.String(e.clusterName),
		Services: []*string{aws.String(serviceName)},
	}

	ecsLogger.Debugf("Waiting for service %v on %v to become stable", serviceName, e.clusterName)

	err := svc.WaitUntilServicesStable(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			ecsLogger.Errorf(aerr.Error())
		} else {
			ecsLogger.Errorf(err.Error())
		}
		return err
	}
	return nil
}
func (e *ECS) launchWaitUntilServicesStable(dd *DynamoDeployment) error {
	service := newService()
	err := e.waitUntilServicesStable(dd.ServiceName)
	ecsLogger.Debugf("Waiting for service %v to become stable finished", dd.ServiceName)
	if err != nil {
		ecsLogger.Debugf("waitUntilServiceStable didn't succeed: %v", err)
		service.setDeploymentStatus(dd, "failed")
	}
	ecsLogger.Debugf("Service %v stable", dd.ServiceName)
	service.setDeploymentStatus(dd, "success")
	return nil
}

// describe services
func (e *ECS) describeServices(clusterName string, serviceNames []*string) ([]RunningService, error) {
	var rss []RunningService
	svc := ecs.New(session.New())

	// fetch per 10
	var y float64 = float64(len(serviceNames)) / 10
	for i := 0; i < int(math.Ceil(y)); i++ {

		f := i * 10
		t := int(math.Min(float64(10+10*i), float64(len(serviceNames))))

		input := &ecs.DescribeServicesInput{
			Cluster:  aws.String(clusterName),
			Services: serviceNames[f:t],
		}

		result, err := svc.DescribeServices(input)
		if err != nil {
			if aerr, ok := err.(awserr.Error); ok {
				ecsLogger.Errorf(aerr.Error())
			} else {
				ecsLogger.Errorf(err.Error())
			}
			return rss, err
		}
		for _, service := range result.Services {
			rs := RunningService{ServiceName: *service.ServiceName, ClusterName: clusterName}
			rs.RunningCount = *service.RunningCount
			rs.Status = *service.Status
			for _, deployment := range service.Deployments {
				var ds RunningServiceDeployment
				ds.Status = *deployment.Status
				ds.RunningCount = *deployment.RunningCount
				ds.PendingCount = *deployment.PendingCount
				ds.DesiredCount = *deployment.DesiredCount
				ds.CreatedAt = *deployment.CreatedAt
				ds.UpdatedAt = *deployment.UpdatedAt
				rs.Deployments = append(rs.Deployments, ds)
			}
			rss = append(rss, rs)
		}
	}
	return rss, nil
}
