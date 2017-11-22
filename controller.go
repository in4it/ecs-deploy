package main

import (
	"github.com/juju/loggo"

	"errors"
	"fmt"
)

// Controller struct
type Controller struct {
}

// logging
var controllerLogger = loggo.GetLogger("controller")

func (c *Controller) createRepository(repository string) (string, error) {
	// create service in ECR if not exists
	ecr := ECR{repositoryName: repository}
	err := ecr.createRepository()
	if err != nil {
		controllerLogger.Errorf("Could not create repository %v: %v", repository, err)
		return fmt.Sprintf("error - Could not create repo: %v\n", repository), errors.New("CouldNotCreateRepository")
	} else {
		// create service in dynamodb
		service := Service{ServiceName: repository, ECRRepositoryURI: ecr.repositoryURI}
		service.createService()
		// return message
		return fmt.Sprintf("Service: %v - ECR: %v", service.ServiceName, service.ECRRepositoryURI), nil
	}
}

func (c *Controller) deploy(serviceName string, d Deploy) (*string, error) {
	// validate
	for _, container := range d.Containers {
		if container.Memory == 0 && container.MemoryReservation == 0 {
			controllerLogger.Errorf("Could not deploy %v: Memory / MemoryReservation not set", serviceName)
			return nil, errors.New("At least one of 'memory' or 'memoryReservation' must be specified within the container specification.")
		}
	}

	// create role if role doesn't exists
	iam := IAM{}
	iamRoleArn, err := iam.roleExists("ecs-" + serviceName)
	if err == nil && iamRoleArn == nil {
		// role does not exist, create it
		controllerLogger.Debugf("Role does not exist, creating: ecs-%v", serviceName)
		iamRoleArn, err = iam.createRole("ecs-"+serviceName, iam.getEcsTaskIAMTrust())
		if err != nil {
			return nil, err
		}
		// optionally add a policy
		ps := Paramstore{}
		if ps.isEnabled() {
			controllerLogger.Debugf("Paramstore enabled, putting role: paramstore-%v", serviceName)
			err = iam.putRolePolicy("ecs-"+serviceName, "paramstore-"+serviceName, ps.getParamstoreIAMPolicy(serviceName))
			if err != nil {
				return nil, err
			}
		}
	} else if err != nil {
		return nil, err
	}

	// create task definition
	ecs := ECS{serviceName: serviceName, iamRoleArn: *iamRoleArn, clusterName: d.Cluster}
	taskDefArn, err := ecs.createTaskDefinition(d)
	if err != nil {
		controllerLogger.Errorf("Could not create task def %v", serviceName)
		return nil, err
	}
	controllerLogger.Debugf("Created task definition: %v", *taskDefArn)
	// check desired instances in dynamodb

	// update service with new task (update desired instance in case of difference)
	controllerLogger.Debugf("Updating service: %v with taskdefarn: %v", serviceName, *taskDefArn)
	serviceExists, err := ecs.serviceExists(serviceName)
	if err == nil && !serviceExists {
		controllerLogger.Debugf("service (%v) not found, creating...", serviceName)
		err = c.createService(serviceName, d, taskDefArn)
		if err != nil {
			controllerLogger.Errorf("Could not create service %v", serviceName)
			return nil, err
		}
	} else if err != nil {
		return nil, errors.New("Error during checking whether service exists")
	} else {
		// update service
		_, err = ecs.updateService(serviceName, taskDefArn)
		controllerLogger.Debugf("Updating ecs service: %v", serviceName)
		if err != nil {
			controllerLogger.Errorf("Could not update service %v: %v", serviceName, err)
			return nil, err
		}
	}

	// write changes in db
	// todo

	ret := fmt.Sprintf("Successfully deployed service %v with taskdefinition %v", serviceName, *taskDefArn)
	return &ret, nil
}

// service not found, create ALB target group + rule
func (c *Controller) createService(serviceName string, d Deploy, taskDefArn *string) error {
	iam := IAM{}
	alb := ALB{}
	alb.init(d.Cluster)

	// create target group
	controllerLogger.Debugf("Creating target group for service: %v", serviceName)
	targetGroupArn, err := alb.createTargetGroup(serviceName, d)
	if err != nil {
		return err
	}

	// deploy rules for target group
	err = c.deployRulesForTarget(serviceName, d, targetGroupArn, &alb)
	if err != nil {
		return err
	}

	// check whether ecs-service-role exists
	controllerLogger.Debugf("Checking whether role exists: %v", getEnv("AWS_ECS_SERVICE_ROLE", "ecs-service-role"))
	iamServiceRoleArn, err := iam.roleExists(getEnv("AWS_ECS_SERVICE_ROLE", "ecs-service-role"))
	if err == nil && iamServiceRoleArn == nil {
		controllerLogger.Debugf("Creating ecs service role")
		_, err = iam.createRole(getEnv("AWS_ECS_SERVICE_ROLE", "ecs-service-role"), iam.getEcsServiceIAMTrust())
		if err != nil {
			return err
		}
		controllerLogger.Debugf("Attaching ecs service role")
		err = iam.attachRolePolicy(getEnv("AWS_ECS_SERVICE_ROLE", "ecs-service-role"), iam.getEcsServicePolicy())
		if err != nil {
			return err
		}
	} else if err != nil {
		return errors.New("Error during checking whether ecs service role exists")
	}

	// create ecs service
	controllerLogger.Debugf("Creating ecs service: %v", serviceName)
	ecs := ECS{serviceName: serviceName, taskDefArn: taskDefArn, targetGroupArn: targetGroupArn}
	err = ecs.createService(d)
	if err != nil {
		return err
	}
	return nil
}

// Deploy rules for a specific targetGroup
func (c *Controller) deployRulesForTarget(serviceName string, d Deploy, targetGroupArn *string, alb *ALB) error {
	// get last priority number
	priority, err := alb.getHighestRule()
	if err != nil {
		return err
	}

	if len(d.RuleConditions) > 0 {
		// create rules based on conditions
		var newRules int
		for _, r := range d.RuleConditions {
			if r.PathPattern != "" && r.Hostname != "" {
				rules := []string{r.PathPattern, r.Hostname}
				err = alb.createRuleForListeners("combined", r.Listeners, *targetGroupArn, rules, (priority + 10 + int64(newRules)))
				if err != nil {
					return err
				}
				newRules += len(r.Listeners)
			} else if r.PathPattern != "" {
				rules := []string{r.PathPattern}
				err = alb.createRuleForListeners("pathPattern", r.Listeners, *targetGroupArn, rules, (priority + 10 + int64(newRules)))
				if err != nil {
					return err
				}
				newRules += len(r.Listeners)
			} else if r.Hostname != "" {
				rules := []string{r.Hostname}
				err = alb.createRuleForListeners("hostname", r.Listeners, *targetGroupArn, rules, (priority + 10 + int64(newRules)))
				if err != nil {
					return err
				}
				newRules += len(r.Listeners)
			}
		}
	} else {
		// create default rules ( /servicename path on all listeners )
		controllerLogger.Debugf("Creating alb rule(s) service: %v", serviceName)
		rules := []string{"/" + serviceName}
		err = alb.createRuleForAllListeners("pathPattern", *targetGroupArn, rules, (priority + 10))
		if err != nil {
			return err
		}
		rules = []string{"/" + serviceName + "/*"}
		err = alb.createRuleForAllListeners("pathPattern", *targetGroupArn, rules, (priority + 11))
		if err != nil {
			return err
		}
	}
	return nil
}
