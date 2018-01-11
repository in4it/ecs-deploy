package main

import (
	"github.com/google/go-cmp/cmp"
	"github.com/juju/loggo"

	"errors"
	"fmt"
	"strings"
)

// Controller struct
type Controller struct {
}

// logging
var controllerLogger = loggo.GetLogger("controller")

func (c *Controller) createRepository(repository string) (*string, error) {
	// create service in ECR if not exists
	ecr := ECR{repositoryName: repository}
	err := ecr.createRepository()
	if err != nil {
		controllerLogger.Errorf("Could not create repository %v: %v", repository, err)
		return nil, errors.New("CouldNotCreateRepository")
	}
	msg := fmt.Sprintf("Service: %v - ECR: %v", repository, ecr.repositoryURI)
	return &msg, nil
}

func (c *Controller) deploy(serviceName string, d Deploy) (*DeployResult, error) {
	// get last deployment
	service := newService()
	service.serviceName = serviceName
	service.clusterName = d.Cluster
	ddLast, err := service.getLastDeploy()
	if err != nil {
		if !strings.HasPrefix(err.Error(), "NoItemsFound") {
			controllerLogger.Errorf("Error while getting last deployment for %v: %v", serviceName, err)
			return nil, err
		}
	}
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
		alb, err := newALB(d.Cluster)
		targetGroupArn, err := alb.getTargetGroupArn(serviceName)
		if err != nil {
			return nil, err
		}
		// update healthchecks if changed
		if !cmp.Equal(ddLast.DeployData.HealthCheck, d.HealthCheck) {
			controllerLogger.Debugf("Updating ecs healthcheck: %v", serviceName)
			alb.updateHealthCheck(*targetGroupArn, d.HealthCheck)
		}
		// update target group attributes if changed
		if !cmp.Equal(ddLast.DeployData.Stickiness, d.Stickiness) || ddLast.DeployData.DeregistrationDelay != d.DeregistrationDelay {
			err = alb.modifyTargetGroupAttributes(*targetGroupArn, d)
			if err != nil {
				return nil, err
			}
		}
		// update service
		_, err = ecs.updateService(serviceName, taskDefArn, d)
		controllerLogger.Debugf("Updating ecs service: %v", serviceName)
		if err != nil {
			controllerLogger.Errorf("Could not update service %v: %v", serviceName, err)
			return nil, err
		}
	}

	// Mark previous deployment as aborted if still running
	if ddLast != nil && ddLast.Status == "running" {
		err = service.setDeploymentStatus(ddLast, "aborted")
		if err != nil {
			controllerLogger.Errorf("Could not set status of %v to aborted: %v", serviceName, err)
			return nil, err
		}
	}

	// write changes in db
	dd, err := service.newDeployment(taskDefArn, &d)
	if err != nil {
		controllerLogger.Errorf("Could not create/update service (%v) in db: %v", serviceName, err)
		return nil, err
	}

	// run goroutine to update status of service
	go ecs.launchWaitUntilServicesStable(dd)

	ret := &DeployResult{
		ServiceName:       serviceName,
		ClusterName:       d.Cluster,
		TaskDefinitionArn: *taskDefArn,
		DeploymentTime:    dd.Time,
	}
	return ret, nil
}
func (c *Controller) redeploy(serviceName, time string) (*DeployResult, error) {
	s := newService()
	dd, err := s.getDeployment(serviceName, time)
	if err != nil {
		return nil, err
	}

	controllerLogger.Debugf("Redeploying %v_%v", serviceName, time)

	ret, err := c.deploy(serviceName, *dd.DeployData)

	if err != nil {
		return nil, err
	}

	return ret, nil
}

// service not found, create ALB target group + rule
func (c *Controller) createService(serviceName string, d Deploy, taskDefArn *string) error {
	iam := IAM{}
	alb, err := newALB(d.Cluster)
	if err != nil {
		return err
	}

	// create target group
	controllerLogger.Debugf("Creating target group for service: %v", serviceName)
	targetGroupArn, err := alb.createTargetGroup(serviceName, d)
	if err != nil {
		return err
	}
	// modify target group attributes
	if d.DeregistrationDelay != -1 || d.Stickiness.Enabled {
		err = alb.modifyTargetGroupAttributes(*targetGroupArn, d)
		if err != nil {
			return err
		}
	}

	// deploy rules for target group
	listeners, err := c.createRulesForTarget(serviceName, d, targetGroupArn, alb)
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

	// create service in dynamodb
	service := newService()
	service.serviceName = serviceName
	service.clusterName = d.Cluster
	service.listeners = listeners

	err = service.createService()
	if err != nil {
		controllerLogger.Errorf("Could not create/update service (%v) in db: %v", serviceName, err)
		return err
	}
	return nil
}

// Deploy rules for a specific targetGroup
func (c *Controller) createRulesForTarget(serviceName string, d Deploy, targetGroupArn *string, alb *ALB) ([]string, error) {
	var listeners []string
	// get last priority number
	priority, err := alb.getHighestRule()
	if err != nil {
		return nil, err
	}

	if len(d.RuleConditions) > 0 {
		// create rules based on conditions
		var newRules int
		for _, r := range d.RuleConditions {
			if r.PathPattern != "" && r.Hostname != "" {
				rules := []string{r.PathPattern, r.Hostname}
				l, err := alb.createRuleForListeners("combined", r.Listeners, *targetGroupArn, rules, (priority + 10 + int64(newRules)))
				if err != nil {
					return nil, err
				}
				newRules += len(r.Listeners)
				listeners = append(listeners, l...)
			} else if r.PathPattern != "" {
				rules := []string{r.PathPattern}
				l, err := alb.createRuleForListeners("pathPattern", r.Listeners, *targetGroupArn, rules, (priority + 10 + int64(newRules)))
				if err != nil {
					return nil, err
				}
				newRules += len(r.Listeners)
				listeners = append(listeners, l...)
			} else if r.Hostname != "" {
				rules := []string{r.Hostname}
				l, err := alb.createRuleForListeners("hostname", r.Listeners, *targetGroupArn, rules, (priority + 10 + int64(newRules)))
				if err != nil {
					return nil, err
				}
				newRules += len(r.Listeners)
				listeners = append(listeners, l...)
			}
		}
	} else {
		// create default rules ( /servicename path on all listeners )
		controllerLogger.Debugf("Creating alb rule(s) service: %v", serviceName)
		rules := []string{"/" + serviceName}
		l, err := alb.createRuleForAllListeners("pathPattern", *targetGroupArn, rules, (priority + 10))
		if err != nil {
			return nil, err
		}
		rules = []string{"/" + serviceName + "/*"}
		_, err = alb.createRuleForAllListeners("pathPattern", *targetGroupArn, rules, (priority + 11))
		if err != nil {
			return nil, err
		}
		listeners = append(listeners, l...)
	}
	return listeners, nil
}

func (c *Controller) getDeploys() ([]DynamoDeployment, error) {
	s := newService()
	return s.getDeploys("byMonth", 20)
}
func (c *Controller) getDeploysForService(serviceName string) ([]DynamoDeployment, error) {
	s := newService()
	return s.getDeploysForService(serviceName)
}
func (c *Controller) getServices() ([]*DynamoServicesElement, error) {
	s := newService()
	var ds DynamoServices
	err := s.getServices(&ds)
	return ds.Services, err
}

func (c *Controller) describeServices() ([]RunningService, error) {
	var rss []RunningService
	showEvents := false
	showTasks := false
	showStoppedTasks := false
	services := make(map[string][]*string)
	ecs := ECS{}
	dss, _ := c.getServices()
	for _, ds := range dss {
		services[ds.C] = append(services[ds.C], &ds.S)
	}
	for clusterName, serviceList := range services {
		newRss, err := ecs.describeServices(clusterName, serviceList, showEvents, showTasks, showStoppedTasks)
		if err != nil {
			return []RunningService{}, err
		}
		rss = append(rss, newRss...)
	}
	return rss, nil
}
func (c *Controller) describeService(serviceName string) (RunningService, error) {
	var rs RunningService
	showEvents := true
	showTasks := true
	showStoppedTasks := false
	ecs := ECS{}
	dss, _ := c.getServices()
	for _, ds := range dss {
		if ds.S == serviceName {
			rss, err := ecs.describeServices(ds.C, []*string{&serviceName}, showEvents, showTasks, showStoppedTasks)
			if err != nil {
				return rs, err
			}
			if len(rss) != 1 {
				return rs, errors.New("Empty RunningService object returned")
			}
			rs = rss[0]
			return rs, nil
		}
	}
	return rs, errors.New("Service " + serviceName + " not found")
}
func (c *Controller) describeServiceVersions(serviceName string) ([]ServiceVersion, error) {
	var imageName string
	var sv []ServiceVersion
	service := newService()
	service.serviceName = serviceName
	ecr := ECR{}
	// get last service to know container name
	ddLast, err := service.getLastDeploy()
	if err != nil {
		return sv, err
	}
	// get image linked with main container
	for _, container := range ddLast.DeployData.Containers {
		if container.ContainerName == serviceName {
			if container.ContainerImage != "" {
				imageName = container.ContainerImage
			} else {
				imageName = serviceName
			}
		}
	}
	if imageName == "" {
		return sv, errors.New("Couldn't find imageName for service " + serviceName)
	}
	// get image tags
	tags, err := ecr.listImagesWithTag(imageName)
	if err != nil {
		return sv, err
	}
	// populate last deployed on
	sv, err = service.getServiceVersionsByTags(serviceName, imageName, tags)
	if err != nil {
		return sv, err
	}
	return sv, nil
}
func (c *Controller) getDeploymentStatus(serviceName, time string) (*DeployResult, error) {
	s := newService()
	dd, err := s.getDeployment(serviceName, time)
	if err != nil {
		return nil, err
	}
	ret := &DeployResult{
		ClusterName:       dd.DeployData.Cluster,
		ServiceName:       serviceName,
		DeploymentTime:    dd.Time,
		Status:            dd.Status,
		TaskDefinitionArn: *dd.TaskDefinitionArn,
	}
	return ret, nil
}
func (c *Controller) getDeployment(serviceName, time string) (*Deploy, error) {
	s := newService()
	dd, err := s.getDeployment(serviceName, time)
	if err != nil {
		return nil, err
	}
	return dd.DeployData, nil
}
func (c *Controller) getServiceParameters(serviceName, userId, creds string) (map[string]Parameter, string, error) {
	var err error
	p := Paramstore{}
	role := getEnv("PARAMSTORE_ASSUME_ROLE", "")
	if role != "" {
		creds, err = p.assumeRole(role, userId, creds)
		if err != nil {
			return p.parameters, creds, err
		}
	}
	err = p.getParameters("/"+getEnv("PARAMSTORE_PREFIX", "")+"-"+getEnv("AWS_ACCOUNT_ENV", "")+"/"+serviceName+"/", false)
	if err != nil {
		return p.parameters, creds, err
	}
	return p.parameters, creds, nil
}
func (c *Controller) putServiceParameter(serviceName, userId, creds string, parameter DeployServiceParameter) (map[string]int64, string, error) {
	var err error
	p := Paramstore{}
	res := make(map[string]int64)
	role := getEnv("PARAMSTORE_ASSUME_ROLE", "")
	if role != "" {
		creds, err = p.assumeRole(role, userId, creds)
		if err != nil {
			return res, creds, err
		}
	}
	version, err := p.putParameter(serviceName, parameter)

	res["version"] = *version

	return res, creds, err
}
func (c *Controller) deleteServiceParameter(serviceName, userId, creds, parameter string) (string, error) {
	var err error
	p := Paramstore{}
	role := getEnv("PARAMSTORE_ASSUME_ROLE", "")
	if role != "" {
		creds, err = p.assumeRole(role, userId, creds)
		if err != nil {
			return creds, err
		}
	}
	err = p.deleteParameter(serviceName, parameter)

	return creds, err
}

func (c *Controller) deleteService(serviceName string) error {
	var ds *DynamoServices
	var clusterName string
	service := Service{}
	err := service.getServices(ds)
	if err != nil {
		return err
	}
	for _, v := range ds.Services {
		if v.S == serviceName {
			clusterName = v.C
		}
	}
	alb, err := newALB(clusterName)
	if err != nil {
		return err
	}
	targetGroupArn, err := alb.getTargetGroupArn(serviceName)
	if err != nil {
		return err
	}
	err = alb.deleteTargetGroup(*targetGroupArn)
	if err != nil {
		return err
	}
	return nil
}
func (c *Controller) scaleService(serviceName string, desiredCount int64) error {
	service := newService()
	service.serviceName = serviceName
	clusterName, err := service.getClusterName()
	if err != nil {
		return err
	}
	service.setScalingProperty(desiredCount)
	ecs := ECS{}
	ecs.manualScaleService(clusterName, serviceName, desiredCount)
	return nil
}

func (c *Controller) setDeployDefaults(d *Deploy) {
	d.DeregistrationDelay = -1
	d.Stickiness.Duration = -1
}
