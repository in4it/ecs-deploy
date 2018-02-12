package api

import (
	"github.com/google/go-cmp/cmp"
	"github.com/in4it/ecs-deploy/provider/ecs"
	"github.com/in4it/ecs-deploy/service"
	"github.com/in4it/ecs-deploy/util"
	"github.com/juju/loggo"

	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
)

// Controller struct
type Controller struct {
}

// logging
var controllerLogger = loggo.GetLogger("controller")

func (c *Controller) createRepository(repository string) (*string, error) {
	// create service in ECR if not exists
	ecr := ecs.ECR{RepositoryName: repository}
	err := ecr.CreateRepository()
	if err != nil {
		controllerLogger.Errorf("Could not create repository %v: %v", repository, err)
		return nil, errors.New("CouldNotCreateRepository")
	}
	msg := fmt.Sprintf("Service: %v - ECR: %v", repository, ecr.RepositoryURI)
	return &msg, nil
}

func (c *Controller) Deploy(serviceName string, d service.Deploy) (*service.DeployResult, error) {
	// get last deployment
	s := service.NewService()
	s.ServiceName = serviceName
	s.ClusterName = d.Cluster
	ddLast, err := s.GetLastDeploy()
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
	iam := ecs.IAM{}
	iamRoleArn, err := iam.RoleExists("ecs-" + serviceName)
	if err == nil && iamRoleArn == nil {
		// role does not exist, create it
		controllerLogger.Debugf("Role does not exist, creating: ecs-%v", serviceName)
		iamRoleArn, err = iam.CreateRole("ecs-"+serviceName, iam.GetEcsTaskIAMTrust())
		if err != nil {
			return nil, err
		}
		// optionally add a policy
		ps := ecs.Paramstore{}
		if ps.IsEnabled() {
			controllerLogger.Debugf("Paramstore enabled, putting role: paramstore-%v", serviceName)
			err = iam.PutRolePolicy("ecs-"+serviceName, "paramstore-"+serviceName, ps.GetParamstoreIAMPolicy(serviceName))
			if err != nil {
				return nil, err
			}
		}
	} else if err != nil {
		return nil, err
	}

	// create task definition
	e := ecs.ECS{ServiceName: serviceName, IamRoleArn: *iamRoleArn, ClusterName: d.Cluster}
	taskDefArn, err := e.CreateTaskDefinition(d)
	if err != nil {
		controllerLogger.Errorf("Could not create task def %v", serviceName)
		return nil, err
	}
	controllerLogger.Debugf("Created task definition: %v", *taskDefArn)
	// check desired instances in dynamodb

	// update service with new task (update desired instance in case of difference)
	controllerLogger.Debugf("Updating service: %v with taskdefarn: %v", serviceName, *taskDefArn)
	serviceExists, err := e.ServiceExists(serviceName)
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
		// compare with previous deployment if there is one
		if ddLast != nil {
			if strings.ToLower(d.ServiceProtocol) != "none" {
				alb, err := ecs.NewALB(d.Cluster)
				targetGroupArn, err := alb.GetTargetGroupArn(serviceName)
				if err != nil {
					return nil, err
				}
				// update healthchecks if changed
				if !cmp.Equal(ddLast.DeployData.HealthCheck, d.HealthCheck) {
					controllerLogger.Debugf("Updating ecs healthcheck: %v", serviceName)
					alb.UpdateHealthCheck(*targetGroupArn, d.HealthCheck)
				}
				// update target group attributes if changed
				if !cmp.Equal(ddLast.DeployData.Stickiness, d.Stickiness) || ddLast.DeployData.DeregistrationDelay != d.DeregistrationDelay {
					err = alb.ModifyTargetGroupAttributes(*targetGroupArn, d)
					if err != nil {
						return nil, err
					}
				}
			}
			// update memory limits if changed
			if !e.IsEqualContainerLimits(d, *ddLast.DeployData) {
				cpuReservation, cpuLimit, memoryReservation, memoryLimit := e.GetContainerLimits(d)
				s.UpdateServiceLimits(s.ClusterName, s.ServiceName, cpuReservation, cpuLimit, memoryReservation, memoryLimit)
			}
		}
		// update service
		_, err = e.UpdateService(serviceName, taskDefArn, d)
		controllerLogger.Debugf("Updating ecs service: %v", serviceName)
		if err != nil {
			controllerLogger.Errorf("Could not update service %v: %v", serviceName, err)
			return nil, err
		}
	}

	// Mark previous deployment as aborted if still running
	if ddLast != nil && ddLast.Status == "running" {
		err = s.SetDeploymentStatus(ddLast, "aborted")
		if err != nil {
			controllerLogger.Errorf("Could not set status of %v to aborted: %v", serviceName, err)
			return nil, err
		}
	}

	// write changes in db
	dd, err := s.NewDeployment(taskDefArn, &d)
	if err != nil {
		controllerLogger.Errorf("Could not create/update service (%v) in db: %v", serviceName, err)
		return nil, err
	}

	// run goroutine to update status of service
	go e.LaunchWaitUntilServicesStable(dd)

	ret := &service.DeployResult{
		ServiceName:       serviceName,
		ClusterName:       d.Cluster,
		TaskDefinitionArn: *taskDefArn,
		DeploymentTime:    dd.Time,
	}
	return ret, nil
}
func (c *Controller) redeploy(serviceName, time string) (*service.DeployResult, error) {
	s := service.NewService()
	dd, err := s.GetDeployment(serviceName, time)
	if err != nil {
		return nil, err
	}

	controllerLogger.Debugf("Redeploying %v_%v", serviceName, time)

	ret, err := c.Deploy(serviceName, *dd.DeployData)

	if err != nil {
		return nil, err
	}

	return ret, nil
}

// service not found, create ALB target group + rule
func (c *Controller) createService(serviceName string, d service.Deploy, taskDefArn *string) error {
	iam := ecs.IAM{}
	var targetGroupArn *string
	var listeners []string
	alb, err := ecs.NewALB(d.Cluster)
	if err != nil {
		return err
	}

	// create target group
	if strings.ToLower(d.ServiceProtocol) != "none" {
		var err error
		controllerLogger.Debugf("Creating target group for service: %v", serviceName)
		targetGroupArn, err = alb.CreateTargetGroup(serviceName, d)
		if err != nil {
			return err
		}
		// modify target group attributes
		if d.DeregistrationDelay != -1 || d.Stickiness.Enabled {
			err = alb.ModifyTargetGroupAttributes(*targetGroupArn, d)
			if err != nil {
				return err
			}
		}

		// deploy rules for target group
		listeners, err = c.createRulesForTarget(serviceName, d, targetGroupArn, alb)
		if err != nil {
			return err
		}
	}

	// check whether ecs-service-role exists
	controllerLogger.Debugf("Checking whether role exists: %v", util.GetEnv("AWS_ECS_SERVICE_ROLE", "ecs-service-role"))
	iamServiceRoleArn, err := iam.RoleExists(util.GetEnv("AWS_ECS_SERVICE_ROLE", "ecs-service-role"))
	if err == nil && iamServiceRoleArn == nil {
		controllerLogger.Debugf("Creating ecs service role")
		_, err = iam.CreateRole(util.GetEnv("AWS_ECS_SERVICE_ROLE", "ecs-service-role"), iam.GetEcsServiceIAMTrust())
		if err != nil {
			return err
		}
		controllerLogger.Debugf("Attaching ecs service role")
		err = iam.AttachRolePolicy(util.GetEnv("AWS_ECS_SERVICE_ROLE", "ecs-service-role"), iam.GetEcsServicePolicy())
		if err != nil {
			return err
		}
	} else if err != nil {
		return errors.New("Error during checking whether ecs service role exists")
	}

	// create ecs service
	controllerLogger.Debugf("Creating ecs service: %v", serviceName)
	e := ecs.ECS{ServiceName: serviceName, TaskDefArn: taskDefArn, TargetGroupArn: targetGroupArn}
	err = e.CreateService(d)
	if err != nil {
		return err
	}

	// create service in dynamodb
	s := service.NewService()
	s.ServiceName = serviceName
	s.ClusterName = d.Cluster
	s.Listeners = listeners

	dsEl := &service.DynamoServicesElement{S: s.ServiceName, C: s.ClusterName, L: s.Listeners}
	dsEl.CpuReservation, dsEl.CpuLimit, dsEl.MemoryReservation, dsEl.MemoryLimit = e.GetContainerLimits(d)

	err = s.CreateService(dsEl)
	if err != nil {
		controllerLogger.Errorf("Could not create/update service (%v) in db: %v", serviceName, err)
		return err
	}
	return nil
}

// Deploy rules for a specific targetGroup
func (c *Controller) createRulesForTarget(serviceName string, d service.Deploy, targetGroupArn *string, alb *ecs.ALB) ([]string, error) {
	var listeners []string
	// get last priority number
	priority, err := alb.GetHighestRule()
	if err != nil {
		return nil, err
	}

	if len(d.RuleConditions) > 0 {
		// create rules based on conditions
		var newRules int
		for _, r := range d.RuleConditions {
			if r.PathPattern != "" && r.Hostname != "" {
				rules := []string{r.PathPattern, r.Hostname}
				l, err := alb.CreateRuleForListeners("combined", r.Listeners, *targetGroupArn, rules, (priority + 10 + int64(newRules)))
				if err != nil {
					return nil, err
				}
				newRules += len(r.Listeners)
				listeners = append(listeners, l...)
			} else if r.PathPattern != "" {
				rules := []string{r.PathPattern}
				l, err := alb.CreateRuleForListeners("pathPattern", r.Listeners, *targetGroupArn, rules, (priority + 10 + int64(newRules)))
				if err != nil {
					return nil, err
				}
				newRules += len(r.Listeners)
				listeners = append(listeners, l...)
			} else if r.Hostname != "" {
				rules := []string{r.Hostname}
				l, err := alb.CreateRuleForListeners("hostname", r.Listeners, *targetGroupArn, rules, (priority + 10 + int64(newRules)))
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
		l, err := alb.CreateRuleForAllListeners("pathPattern", *targetGroupArn, rules, (priority + 10))
		if err != nil {
			return nil, err
		}
		rules = []string{"/" + serviceName + "/*"}
		_, err = alb.CreateRuleForAllListeners("pathPattern", *targetGroupArn, rules, (priority + 11))
		if err != nil {
			return nil, err
		}
		listeners = append(listeners, l...)
	}
	return listeners, nil
}

func (c *Controller) getDeploys() ([]service.DynamoDeployment, error) {
	s := service.NewService()
	return s.GetDeploys("byMonth", 20)
}
func (c *Controller) getDeploysForService(serviceName string) ([]service.DynamoDeployment, error) {
	s := service.NewService()
	return s.GetDeploysForService(serviceName)
}
func (c *Controller) getServices() ([]*service.DynamoServicesElement, error) {
	s := service.NewService()
	var ds service.DynamoServices
	err := s.GetServices(&ds)
	return ds.Services, err
}

func (c *Controller) describeServices() ([]service.RunningService, error) {
	var rss []service.RunningService
	showEvents := false
	showTasks := false
	showStoppedTasks := false
	services := make(map[string][]*string)
	e := ecs.ECS{}
	dss, _ := c.getServices()
	for _, ds := range dss {
		services[ds.C] = append(services[ds.C], &ds.S)
	}
	for clusterName, serviceList := range services {
		newRss, err := e.DescribeServices(clusterName, serviceList, showEvents, showTasks, showStoppedTasks)
		if err != nil {
			return []service.RunningService{}, err
		}
		rss = append(rss, newRss...)
	}
	return rss, nil
}
func (c *Controller) describeService(serviceName string) (service.RunningService, error) {
	var rs service.RunningService
	showEvents := true
	showTasks := true
	showStoppedTasks := false
	e := ecs.ECS{}
	dss, _ := c.getServices()
	for _, ds := range dss {
		if ds.S == serviceName {
			rss, err := e.DescribeServices(ds.C, []*string{&serviceName}, showEvents, showTasks, showStoppedTasks)
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
func (c *Controller) describeServiceVersions(serviceName string) ([]service.ServiceVersion, error) {
	var imageName string
	var sv []service.ServiceVersion
	s := service.NewService()
	s.ServiceName = serviceName
	ecr := ecs.ECR{}
	// get last service to know container name
	ddLast, err := s.GetLastDeploy()
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
	tags, err := ecr.ListImagesWithTag(imageName)
	if err != nil {
		return sv, err
	}
	// populate last deployed on
	sv, err = s.GetServiceVersionsByTags(serviceName, imageName, tags)
	if err != nil {
		return sv, err
	}
	return sv, nil
}
func (c *Controller) getDeploymentStatus(serviceName, time string) (*service.DeployResult, error) {
	s := service.NewService()
	dd, err := s.GetDeployment(serviceName, time)
	if err != nil {
		return nil, err
	}
	ret := &service.DeployResult{
		ClusterName:       dd.DeployData.Cluster,
		ServiceName:       serviceName,
		DeploymentTime:    dd.Time,
		Status:            dd.Status,
		DeployError:       dd.DeployError,
		TaskDefinitionArn: *dd.TaskDefinitionArn,
	}
	return ret, nil
}
func (c *Controller) getDeployment(serviceName, time string) (*service.Deploy, error) {
	s := service.NewService()
	dd, err := s.GetDeployment(serviceName, time)
	if err != nil {
		return nil, err
	}
	return dd.DeployData, nil
}
func (c *Controller) getServiceParameters(serviceName, userId, creds string) (map[string]ecs.Parameter, string, error) {
	var err error
	p := ecs.Paramstore{}
	role := util.GetEnv("PARAMSTORE_ASSUME_ROLE", "")
	if role != "" {
		creds, err = p.AssumeRole(role, userId, creds)
		if err != nil {
			return p.Parameters, creds, err
		}
	}
	err = p.GetParameters("/"+util.GetEnv("PARAMSTORE_PREFIX", "")+"-"+util.GetEnv("AWS_ACCOUNT_ENV", "")+"/"+serviceName+"/", false)
	if err != nil {
		return p.Parameters, creds, err
	}
	return p.Parameters, creds, nil
}
func (c *Controller) putServiceParameter(serviceName, userId, creds string, parameter service.DeployServiceParameter) (map[string]int64, string, error) {
	var err error
	p := ecs.Paramstore{}
	res := make(map[string]int64)
	role := util.GetEnv("PARAMSTORE_ASSUME_ROLE", "")
	if role != "" {
		creds, err = p.AssumeRole(role, userId, creds)
		if err != nil {
			return res, creds, err
		}
	}
	version, err := p.PutParameter(serviceName, parameter)

	res["version"] = *version

	return res, creds, err
}
func (c *Controller) deleteServiceParameter(serviceName, userId, creds, parameter string) (string, error) {
	var err error
	p := ecs.Paramstore{}
	role := util.GetEnv("PARAMSTORE_ASSUME_ROLE", "")
	if role != "" {
		creds, err = p.AssumeRole(role, userId, creds)
		if err != nil {
			return creds, err
		}
	}
	err = p.DeleteParameter(serviceName, parameter)

	return creds, err
}

func (c *Controller) deleteService(serviceName string) error {
	var ds *service.DynamoServices
	var clusterName string
	s := service.NewService()
	err := s.GetServices(ds)
	if err != nil {
		return err
	}
	for _, v := range ds.Services {
		if v.S == serviceName {
			clusterName = v.C
		}
	}
	alb, err := ecs.NewALB(clusterName)
	if err != nil {
		return err
	}
	targetGroupArn, err := alb.GetTargetGroupArn(serviceName)
	if err != nil {
		return err
	}
	err = alb.DeleteTargetGroup(*targetGroupArn)
	if err != nil {
		return err
	}
	return nil
}
func (c *Controller) scaleService(serviceName string, desiredCount int64) error {
	s := service.NewService()
	s.ServiceName = serviceName
	clusterName, err := s.GetClusterName()
	if err != nil {
		return err
	}
	s.SetScalingProperty(desiredCount)
	e := ecs.ECS{}
	e.ManualScaleService(clusterName, serviceName, desiredCount)
	return nil
}

func (c *Controller) SetDeployDefaults(d *service.Deploy) {
	d.DeregistrationDelay = -1
	d.Stickiness.Duration = -1
}

func (c *Controller) runTask(serviceName string, runTask service.RunTask) (string, error) {
	s := service.NewService()
	s.ServiceName = serviceName
	var taskArn string
	clusterName, err := s.GetClusterName()
	if err != nil {
		return taskArn, err
	}
	e := ecs.ECS{}
	taskDefinition, err := e.GetTaskDefinition(clusterName, serviceName)
	if err != nil {
		return taskArn, err
	}
	taskArn, err = e.RunTask(clusterName, taskDefinition, runTask)
	if err != nil {
		return taskArn, err
	}
	err = s.SetManualTasksArn(taskArn)
	if err != nil {
		return taskArn, err
	}
	return taskArn, nil
}
func (c *Controller) describeTaskDefinition(serviceName string) (ecs.TaskDefinition, error) {
	var taskDefinition ecs.TaskDefinition
	s := service.NewService()
	s.ServiceName = serviceName
	clusterName, err := s.GetClusterName()
	if err != nil {
		return taskDefinition, err
	}
	e := ecs.ECS{}
	taskDefinitionName, err := e.GetTaskDefinition(clusterName, serviceName)
	if err != nil {
		return taskDefinition, err
	}
	taskDefinition, err = e.DescribeTaskDefinition(taskDefinitionName)
	if err != nil {
		return taskDefinition, err
	}
	return taskDefinition, nil
}
func (c *Controller) listTasks(serviceName string) ([]service.RunningTask, error) {
	var tasks []service.RunningTask
	var taskArns []*string
	s := service.NewService()
	s.ServiceName = serviceName
	clusterName, err := s.GetClusterName()
	if err != nil {
		return tasks, err
	}
	e := ecs.ECS{}
	runningTasks, err := e.ListTasks(clusterName, serviceName, "RUNNING", "family")
	if err != nil {
		return tasks, err
	}
	stoppedTasks, err := e.ListTasks(clusterName, serviceName, "STOPPED", "family")
	if err != nil {
		return tasks, err
	}
	taskArns = append(taskArns, runningTasks...)
	taskArns = append(taskArns, stoppedTasks...)
	tasks, err = e.DescribeTasks(clusterName, taskArns)
	if err != nil {
		return tasks, err
	}
	return tasks, nil
}
func (c *Controller) getServiceLogs(serviceName, taskArn, containerName string, start, end time.Time) (ecs.CloudWatchLog, error) {
	cw := ecs.CloudWatch{}
	return cw.GetLogEventsByTime(util.GetEnv("CLOUDWATCH_LOGS_PREFIX", "")+"-"+util.GetEnv("AWS_ACCOUNT_ENV", ""), containerName+"/"+containerName+"/"+taskArn, start, end, "")
}

func (c *Controller) Resume() error {
	migration := Migration{}
	s := service.NewService()
	// check api version of database
	dbApiVersion, err := s.GetApiVersion()
	if err != nil {
		return err
	}
	if dbApiVersion != migration.getApiVersion() {
		err := migration.run(dbApiVersion)
		if err != nil {
			return err
		}
	}

	// check whether anything needs to be resumed
	e := ecs.ECS{}
	dds, err := s.GetDeploys("byDay", 20)
	if err != nil {
		return err
	}
	for i, dd := range dds {
		if dd.Status == "running" {
			// run goroutine to update status of service
			controllerLogger.Infof("Starting waitUntilServiceStable for %v", dd.ServiceName)
			go e.LaunchWaitUntilServicesStable(&dds[i])
		}
	}
	// check for nodes draining
	autoscaling := ecs.AutoScaling{}
	services := make(map[string][]string)
	dss, _ := c.getServices()
	for i, ds := range dss {
		services[ds.C] = append(services[ds.C], dss[i].S)
	}
	for clusterName, _ := range services {
		var clusterNotFound bool
		autoScalingGroupName, err := autoscaling.GetAutoScalingGroupByTag(clusterName)
		if err != nil {
			if strings.HasPrefix(err.Error(), "ClusterNotFound:") {
				controllerLogger.Infof("Cluster %v not running - skipping resume for this cluster", clusterName)
				clusterNotFound = true
			} else {
				return err
			}
		}
		if !clusterNotFound {
			var lifecycleHookNotFound bool
			hn, err := autoscaling.GetLifecycleHookNames(autoScalingGroupName, "autoscaling:EC2_INSTANCE_TERMINATING")
			if err != nil || len(hn) == 0 {
				controllerLogger.Errorf("Cluster %v doesn't have a lifecycle hook", clusterName)
				lifecycleHookNotFound = true
			}
			if !lifecycleHookNotFound {
				ciArns, err := e.ListContainerInstances(clusterName)
				if err != nil {
					return err
				}
				cis, err := e.DescribeContainerInstances(clusterName, ciArns)
				if err != nil {
					return err
				}
				dc, err := s.GetClusterInfo()
				if err != nil {
					return err
				}
				for _, ci := range cis {
					if ci.Status == "DRAINING" {
						// write new record to switch container instance to draining (in case there's a record left with DRAINING)
						var writeRecord bool
						if dc != nil {
							for i, dcci := range dc.ContainerInstances {
								if clusterName == dcci.ClusterName && ci.Ec2InstanceId == dcci.ContainerInstanceId && dcci.Status != "DRAINING" {
									dc.ContainerInstances[i].Status = "DRAINING"
									writeRecord = true
								}
							}
						}
						if writeRecord {
							s.PutClusterInfo(*dc, clusterName, "no")
						}
						// launch wait for drained
						controllerLogger.Infof("Launching waitForDrainedNode for cluster=%v, instance=%v, autoscalingGroupName=%v", clusterName, ci.Ec2InstanceId, autoScalingGroupName)
						go e.LaunchWaitForDrainedNode(clusterName, ci.ContainerInstanceArn, ci.Ec2InstanceId, autoScalingGroupName, hn[0], "")
					}
				}
			}
		}
	}
	controllerLogger.Debugf("Finished controller resume. Checked %d services", len(dds))
	return err
}

// Process ECS event message and determine to scale or not
func (c *Controller) processEcsMessage(message ecs.SNSPayloadEcs) error {
	apiLogger.Debugf("found ecs notification")
	s := service.NewService()
	e := ecs.ECS{}
	autoscaling := ecs.AutoScaling{}
	// determine cluster name
	sp := strings.Split(message.Detail.ClusterArn, "/")
	if len(sp) != 2 {
		return errors.New("Could not determine cluster name from message (arn: " + message.Detail.ClusterArn + ")")
	}
	clusterName := sp[1]
	// determine max reservation
	dss, _ := c.getServices()
	memoryNeeded := make(map[string]int64)
	cpuNeeded := make(map[string]int64)
	for _, ds := range dss {
		if val, ok := memoryNeeded[ds.C]; ok {
			if ds.MemoryReservation > val {
				memoryNeeded[ds.C] = ds.MemoryReservation
			}
		} else {
			memoryNeeded[ds.C] = ds.MemoryReservation
		}
		if val, ok := cpuNeeded[ds.C]; ok {
			if ds.CpuReservation > val {
				cpuNeeded[ds.C] = ds.CpuReservation
			}
		} else {
			cpuNeeded[ds.C] = ds.CpuReservation
		}
	}
	if _, ok := memoryNeeded[clusterName]; !ok {
		return errors.New("Minimal Memory needed for clusterName " + clusterName + " not found")
	}
	if _, ok := cpuNeeded[clusterName]; !ok {
		return errors.New("Minimal CPU needed for clusterName " + clusterName + " not found")
	}
	// determine minimum reservations
	dc, err := s.GetClusterInfo()
	if err != nil {
		return err
	}
	if dc == nil || dc.Time.Before(time.Now().Add(-4*time.Minute /* 4 minutes cache */)) {
		// no cache, need to retrieve everything
		controllerLogger.Debugf("No cache found, need to retrieve using API calls")
		dc = &service.DynamoCluster{}
		for k, _ := range memoryNeeded {
			firs, err := e.GetFreeResources(k)
			if err != nil {
				return err
			}
			for _, f := range firs {
				var dcci service.DynamoClusterContainerInstance
				dcci.ClusterName = k
				dcci.ContainerInstanceId = f.InstanceId
				dcci.AvailabilityZone = f.AvailabilityZone
				dcci.FreeMemory = f.FreeMemory
				dcci.FreeCpu = f.FreeCpu
				dcci.Status = f.Status
				dc.ContainerInstances = append(dc.ContainerInstances, dcci)
			}
		}
	}
	var found bool
	for k, v := range dc.ContainerInstances {
		if v.ContainerInstanceId == message.Detail.Ec2InstanceId {
			found = true
			dc.ContainerInstances[k].ClusterName = clusterName
			// get resources
			f, err := e.ConvertResourceToFir(message.Detail.RemainingResources)
			if err != nil {
				return err
			}
			dc.ContainerInstances[k].FreeMemory = f.FreeMemory
			dc.ContainerInstances[k].FreeCpu = f.FreeCpu
			// get az
			for _, v := range message.Detail.Attributes {
				if v.Name == "ecs.availability-zone" {
					dc.ContainerInstances[k].AvailabilityZone = v.Value
				}
			}
		}
	}
	if !found {
		// add element
		var dcci service.DynamoClusterContainerInstance
		dcci.ClusterName = clusterName
		dcci.ContainerInstanceId = message.Detail.Ec2InstanceId
		f, err := e.ConvertResourceToFir(message.Detail.RemainingResources)
		if err != nil {
			return err
		}
		dcci.FreeMemory = f.FreeMemory
		dcci.FreeCpu = f.FreeCpu
		dcci.Status = f.Status
		// get az
		for _, v := range message.Detail.Attributes {
			if v.Name == "ecs.availability-zone" {
				dcci.AvailabilityZone = v.Value
			}
		}
		dc.ContainerInstances = append(dc.ContainerInstances, dcci)
	}
	// check whether at min/max capacity
	autoScalingGroupName, err := autoscaling.GetAutoScalingGroupByTag(clusterName)
	if err != nil {
		return err
	}
	minSize, desiredCapacity, maxSize, err := autoscaling.GetClusterNodeDesiredCount(autoScalingGroupName)
	if err != nil {
		return err
	}
	// make scaling (up) decision
	resourcesFit := make(map[string]bool)
	resourcesFitGlobal := true
	var scalingOp = "no"
	if desiredCapacity < maxSize {
		for _, dcci := range dc.ContainerInstances {
			if dcci.Status != "DRAINING" && dcci.FreeCpu > cpuNeeded[clusterName] && dcci.FreeMemory > memoryNeeded[clusterName] {
				resourcesFit[dcci.AvailabilityZone] = true
				controllerLogger.Debugf("Cluster %v needs at least %v cpu and %v memory. Found instance %v (%v) with %v cpu and %v memory",
					clusterName,
					cpuNeeded[clusterName],
					memoryNeeded[clusterName],
					dcci.ContainerInstanceId,
					dcci.AvailabilityZone,
					dcci.FreeCpu,
					dcci.FreeMemory,
				)
			} else {
				// set resourcesFit[az] in case it's not set to true
				if _, ok := resourcesFit[dcci.AvailabilityZone]; !ok {
					resourcesFit[dcci.AvailabilityZone] = false
				}
			}
		}
		for k, v := range resourcesFit {
			if !v {
				resourcesFitGlobal = false
				controllerLogger.Infof("No instance found in %v with %v cpu and %v memory free", k, cpuNeeded[clusterName], memoryNeeded[clusterName])
			}
		}
		if !resourcesFitGlobal {
			startTime := time.Now().Add(-5 * time.Minute)
			lastScalingOp, err := s.GetScalingActivity(clusterName, startTime)
			if err != nil {
				return err
			}
			if lastScalingOp == "no" {
				controllerLogger.Infof("Initiating scaling activity")
				scalingOp = "up"
				err = autoscaling.ScaleClusterNodes(autoScalingGroupName, 1)
				if err != nil {
					return err
				}
			}
		}
	}
	// make scaling (down) decision
	if desiredCapacity > minSize && resourcesFitGlobal {
		// calculate registered resources
		f, err := e.ConvertResourceToRir(message.Detail.RegisteredResources)
		if err != nil {
			return err
		}
		var clusterMemoryNeeded = f.RegisteredMemory + memoryNeeded[clusterName]        // capacity of full container node + biggest task
		clusterMemoryNeeded += int64(math.Ceil(float64(memoryNeeded[clusterName]) / 2)) // + buffer
		var clusterCpuNeeded = f.RegisteredCpu + cpuNeeded[clusterName]
		totalFreeCpu := make(map[string]int64)
		totalFreeMemory := make(map[string]int64)
		hasFreeResources := make(map[string]bool)
		hasFreeResourcesGlobal := true
		for _, dcci := range dc.ContainerInstances {
			totalFreeCpu[dcci.AvailabilityZone] += dcci.FreeCpu
			totalFreeMemory[dcci.AvailabilityZone] += dcci.FreeMemory
		}
		for k, _ := range totalFreeCpu {
			controllerLogger.Debugf("%v: Have %d cpu available, need %d", k, totalFreeCpu[k], clusterCpuNeeded)
			controllerLogger.Debugf("%v: Have %d memory available, need %d", k, totalFreeMemory[k], clusterMemoryNeeded)
			if totalFreeCpu[k] >= clusterCpuNeeded && totalFreeMemory[k] >= clusterMemoryNeeded {
				hasFreeResources[k] = true
			} else {
				// set hasFreeResources[k] in case the map key hasn't been set to true
				if _, ok := hasFreeResources[k]; !ok {
					hasFreeResources[k] = false
				}
			}
		}
		for _, v := range hasFreeResources {
			if !v {
				hasFreeResourcesGlobal = false
			}
		}
		if hasFreeResourcesGlobal {
			startTime := time.Now().Add(-5 * time.Minute)
			lastScalingOp, err := s.GetScalingActivity(clusterName, startTime)
			if err != nil {
				return err
			}
			if lastScalingOp == "no" {
				controllerLogger.Infof("Starting scaling down operation (cpu: %d >= %d, mem: %d >= %d", totalFreeCpu, clusterCpuNeeded, totalFreeMemory, clusterMemoryNeeded)
				scalingOp = "down"
				autoScalingGroupName, err := autoscaling.GetAutoScalingGroupByTag(clusterName)
				if err != nil {
					return err
				}
				err = autoscaling.ScaleClusterNodes(autoScalingGroupName, -1)
				if err != nil {
					return err
				}
			}
		}
	}
	// write object
	s.PutClusterInfo(*dc, clusterName, scalingOp)
	return nil
}
func (c *Controller) processLifecycleMessage(message ecs.SNSPayloadLifecycle) error {
	e := ecs.ECS{}
	clusterName, err := e.GetClusterNameByInstanceId(message.Detail.EC2InstanceId)
	if err != nil {
		return err
	}
	containerInstanceArn, err := e.GetContainerInstanceArnByInstanceId(clusterName, message.Detail.EC2InstanceId)
	if err != nil {
		return err
	}
	err = e.DrainNode(clusterName, containerInstanceArn)
	if err != nil {
		return err
	}
	s := service.NewService()
	dc, err := s.GetClusterInfo()
	if err != nil {
		return err
	}
	// write new record to switch container instance to draining
	var writeRecord bool
	if dc != nil {
		for i, dcci := range dc.ContainerInstances {
			if clusterName == dcci.ClusterName && message.Detail.EC2InstanceId == dcci.ContainerInstanceId {
				dc.ContainerInstances[i].Status = "DRAINING"
				writeRecord = true
			}
		}
	}
	if writeRecord {
		s.PutClusterInfo(*dc, clusterName, "no")
	}
	// monitor drained node
	go e.LaunchWaitForDrainedNode(clusterName, containerInstanceArn, message.Detail.EC2InstanceId, message.Detail.AutoScalingGroupName, message.Detail.LifecycleHookName, message.Detail.LifecycleActionToken)
	return nil
}

func (c *Controller) Bootstrap(b *Flags) error {
	var ecsDeploy = service.Deploy{
		Cluster:               b.ClusterName,
		ServiceName:           "ecs-deploy",
		ServicePort:           8080,
		ServiceProtocol:       "HTTP",
		DesiredCount:          1,
		MinimumHealthyPercent: 100,
		MaximumPercent:        200,
		Containers: []*service.DeployContainer{
			{
				ContainerName:     "ecs-deploy",
				ContainerPort:     8080,
				ContainerImage:    "ecs-deploy",
				ContainerURI:      "index.docker.io/in4it/ecs-deploy:latest",
				Essential:         true,
				MemoryReservation: 128,
				CPUReservation:    64,
				Environment: []*service.DeployContainerEnvironment{
					{
						Name:  "PARAMSTORE_ENABLED",
						Value: "yes",
					},
				},
			},
		},
		HealthCheck: service.DeployHealthCheck{
			HealthyThreshold:   3,
			UnhealthyThreshold: 3,
			Path:               "/ecs-deploy/health",
		},
	}
	e := ecs.ECS{}
	iam := ecs.IAM{}
	paramstore := ecs.Paramstore{}
	s := service.NewService()
	cloudwatch := ecs.CloudWatch{}
	autoscaling := ecs.AutoScaling{}
	roleName := "ecs-" + b.ClusterName
	instanceProfile := "ecs-" + b.ClusterName
	deployPassword := util.RandStringBytesMaskImprSrc(8)

	// create dynamodb table
	err := s.CreateTable()
	if err != nil && !strings.HasPrefix(err.Error(), "ResourceInUseException") {
		return err
	}

	// create instance profile for cluster
	err = iam.GetAccountId()
	if err != nil {
		return err
	}
	_, err = iam.CreateRole(roleName, iam.GetEC2IAMTrust())
	if err != nil {
		return err
	}
	var ec2RolePolicy string
	if b.CloudwatchLogsEnabled {
		r, err := ioutil.ReadFile("templates/iam/ecs-ec2-policy-logs.json")
		if err != nil {
			return err
		}
		ec2RolePolicy = strings.Replace(string(r), "${LOGS_RESOURCE}", "arn:aws:logs:"+b.Region+":"+iam.AccountId+":log-group:"+b.CloudwatchLogsPrefix+"-"+b.Environment+":*", -1)
	} else {
		r, err := ioutil.ReadFile("templates/iam/ecs-ec2-policy.json")
		if err != nil {
			return err
		}
		ec2RolePolicy = string(r)
	}
	iam.PutRolePolicy(roleName, "ecs-ec2-policy", ec2RolePolicy)

	// wait for role instance profile to exist
	err = iam.CreateInstanceProfile(roleName)
	if err != nil {
		return err
	}
	err = iam.AddRoleToInstanceProfile(roleName, roleName)
	if err != nil {
		return err
	}
	fmt.Println("Waiting until instance profile exists...")
	err = iam.WaitUntilInstanceProfileExists(roleName)
	if err != nil {
		return err
	}
	// import key
	r, err := ioutil.ReadFile(util.GetEnv("HOME", "") + "/.ssh/" + b.KeyName)
	if err != nil {
		return err
	}
	pubKey, err := e.GetPubKeyFromPrivateKey(string(r))
	if err != nil {
		return err
	}
	e.ImportKeyPair(b.ClusterName, pubKey)

	// create launch configuration
	err = autoscaling.CreateLaunchConfiguration(b.ClusterName, b.KeyName, b.InstanceType, instanceProfile, strings.Split(b.EcsSecurityGroups, ","))
	if err != nil {
		for i := 0; i < 5 && err != nil; i++ {
			if strings.HasPrefix(err.Error(), "RetryableError:") {
				fmt.Printf("Error: %v - waiting 10s and retrying...\n", err.Error())
				time.Sleep(10 * time.Second)
				err = autoscaling.CreateLaunchConfiguration(b.ClusterName, b.KeyName, b.InstanceType, instanceProfile, strings.Split(b.EcsSecurityGroups, ","))
			}
		}
		if err != nil {
			return err
		}
	}

	// create autoscaling group
	intEcsDesiredSize, _ := strconv.ParseInt(b.EcsDesiredSize, 10, 64)
	intEcsMaxSize, _ := strconv.ParseInt(b.EcsMaxSize, 10, 64)
	intEcsMinSize, _ := strconv.ParseInt(b.EcsMinSize, 10, 64)
	autoscaling.CreateAutoScalingGroup(b.ClusterName, intEcsDesiredSize, intEcsMaxSize, intEcsMinSize, strings.Split(b.EcsSubnets, ","))
	if err != nil {
		return err
	}

	// create log group
	if b.CloudwatchLogsEnabled {
		err = cloudwatch.CreateLogGroup(b.ClusterName, b.CloudwatchLogsPrefix+"-"+b.Environment)
		if err != nil {
			return err
		}
	}
	// create cluster
	clusterArn, err := e.CreateCluster(b.ClusterName)
	if err != nil {
		return err
	}
	fmt.Printf("Created ECS Cluster with ARN: %v\n", *clusterArn)
	if b.AlbSecurityGroups == "" || b.EcsSubnets == "" {
		return errors.New("Incorrect test arguments supplied")
	}

	// create load balancer, default target, and listener
	alb, err := ecs.NewALBAndCreate(b.ClusterName, "ipv4", "internet-facing", strings.Split(b.AlbSecurityGroups, ","), strings.Split(b.EcsSubnets, ","), "application")
	if err != nil {
		return err
	}
	defaultTargetGroupArn, err := alb.CreateTargetGroup("ecs-deploy", ecsDeploy /* ecs deploy object */)
	if err != nil {
		return err
	}
	err = alb.CreateListener("HTTP", 80, *defaultTargetGroupArn)
	if err != nil {
		return err
	}
	// create env vars
	if b.ParamstoreEnabled {
		parameters := []service.DeployServiceParameter{
			{Name: "PARAMSTORE_ENABLED", Value: "yes"},
			{Name: "PARAMSTORE_PREFIX", Value: b.ParamstorePrefix},
			{Name: "JWT_SECRET", Value: util.RandStringBytesMaskImprSrc(32)},
			{Name: "DEPLOY_PASSWORD", Value: deployPassword},
			{Name: "URL_PREFIX", Value: "/ecs-deploy"},
		}
		if b.ParamstoreKmsArn != "" {
			parameters = append(parameters, service.DeployServiceParameter{Name: "PARAMSTORE_KMS_ARN", Value: b.ParamstoreKmsArn})
		}
		if b.CloudwatchLogsEnabled {
			parameters = append(parameters, service.DeployServiceParameter{Name: "CLOUDWATCH_LOGS_ENABLED", Value: "yes"})
			parameters = append(parameters, service.DeployServiceParameter{Name: "CLOUDWATCH_LOGS_PREFIX", Value: b.CloudwatchLogsPrefix})
		}
		paramstore.Bootstrap("ecs-deploy", b.ParamstorePrefix, b.Environment, parameters)
		// retrieve keys from parameter store and set as environment variable
		os.Setenv("PARAMSTORE_ENABLED", "yes")
		err = paramstore.RetrieveKeys()
		if err != nil {
			return err
		}
	}

	// wait for autoscaling group to be in service
	fmt.Println("Waiting for autoscaling group to be in service...")
	err = autoscaling.WaitForAutoScalingGroupInService(b.ClusterName)
	if err != nil {
		return err
	}
	if !b.DisableEcsDeploy {
		iamRoleArn, err := iam.RoleExists("ecs-ecs-deploy")
		if err == nil && iamRoleArn == nil {
			_, err := iam.CreateRole("ecs-ecs-deploy", iam.GetEcsTaskIAMTrust())
			if err != nil {
				return err
			}
		}
		r, err := ioutil.ReadFile("templates/iam/ecs-deploy-task.json")
		if err != nil {
			return err
		}
		ecsDeployRolePolicy := strings.Replace(string(r), "${ACCOUNT_ID}", iam.AccountId, -1)
		ecsDeployRolePolicy = strings.Replace(ecsDeployRolePolicy, "${AWS_REGION}", b.Region, -1)
		err = iam.PutRolePolicy("ecs-ecs-deploy", "ecs-deploy", ecsDeployRolePolicy)
		if err != nil {
			return err
		}
		_, err = c.Deploy(ecsDeploy.ServiceName, ecsDeploy)
		s.ServiceName = ecsDeploy.ServiceName
		var deployed bool
		for i := 0; i < 30 && !deployed; i++ {
			dd, err := s.GetLastDeploy()
			if err != nil {
				return err
			}
			if dd != nil && dd.Status == "success" {
				deployed = true
			} else if dd != nil && dd.Status == "failed" {
				return errors.New("Deployment of ecs-deploy failed")
			} else {
				fmt.Printf("Waiting for %v to to be deployed (status: %v)\n", ecsDeploy.ServiceName, dd.Status)
				time.Sleep(30 * time.Second)
			}
		}
	}
	fmt.Println("")
	fmt.Println("===============================================")
	fmt.Println("=== Successfully bootstrapped ecs-deploy    ===")
	fmt.Println("===============================================")
	fmt.Printf("     URL: http://%v/ecs-deploy                  \n", alb.DnsName)
	fmt.Printf("     Login: deploy                              \n")
	fmt.Printf("     Password: %v                               \n", deployPassword)
	fmt.Println("===============================================")
	fmt.Println("")
	return nil
}

func (c *Controller) DeleteCluster(b *Flags) error {
	iam := ecs.IAM{}
	e := ecs.ECS{}
	autoscaling := ecs.AutoScaling{}
	clusterName := b.ClusterName
	roleName := "ecs-" + clusterName
	cloudwatch := ecs.CloudWatch{}
	err := autoscaling.DeleteAutoScalingGroup(clusterName, true)
	if err != nil {
		return err
	}
	err = autoscaling.DeleteLaunchConfiguration(clusterName)
	if err != nil {
		return err
	}
	err = e.DeleteKeyPair(clusterName)
	if err != nil {
		return err
	}
	err = iam.DeleteRolePolicy(roleName, "ecs-ec2-policy")
	if err != nil {
		return err
	}
	err = iam.RemoveRoleFromInstanceProfile(roleName, roleName)
	if err != nil {
		return err
	}
	err = iam.DeleteInstanceProfile(roleName)
	if err != nil {
		return err
	}
	err = iam.DeleteRole(roleName)
	if err != nil {
		return err
	}
	alb, err := ecs.NewALB(clusterName)
	if err != nil {
		return err
	}
	for _, v := range alb.Listeners {
		err = alb.DeleteListener(*v.ListenerArn)
		if err != nil {
			return err
		}
	}
	serviceArns, err := e.ListServices(clusterName)
	if err != nil {
		return err
	}
	services, err := e.DescribeServices(clusterName, serviceArns, false, false, false)
	for _, v := range services {
		targetGroup, _ := alb.GetTargetGroupArn(v.ServiceName)
		if targetGroup != nil {
			err = alb.DeleteTargetGroup(*targetGroup)
			if err != nil {
				return err
			}
		}
		err = e.DeleteService(clusterName, v.ServiceName)
		if err != nil {
			return err
		}
		err = e.WaitUntilServicesInactive(clusterName, v.ServiceName)
		if err != nil {
			return err
		}
	}
	err = alb.DeleteLoadBalancer()
	if err != nil {
		return err
	}
	fmt.Println("Wait for autoscaling group to not exist")
	err = autoscaling.WaitForAutoScalingGroupNotExists(clusterName)
	if err != nil {
		return err
	}
	var drained bool
	fmt.Println("Waiting for EC2 instances to drain from ECS cluster")
	for i := 0; i < 5 && !drained; i++ {
		instanceArns, err := e.ListContainerInstances(clusterName)
		if err != nil {
			return err
		}
		if len(instanceArns) == 0 {
			drained = true
		} else {
			time.Sleep(5 * time.Second)
		}
	}
	err = e.DeleteCluster(clusterName)
	if err != nil {
		return err
	}
	err = cloudwatch.DeleteLogGroup(b.CloudwatchLogsPrefix + "-" + b.Environment)
	if err != nil {
		return err
	}
	return nil
}
