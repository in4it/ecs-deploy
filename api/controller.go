package api

import (
	"github.com/google/go-cmp/cmp"
	"github.com/in4it/ecs-deploy/integrations"
	"github.com/in4it/ecs-deploy/provider/ecs"
	"github.com/in4it/ecs-deploy/service"
	"github.com/in4it/ecs-deploy/util"
	"github.com/juju/loggo"

	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
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
		if util.GetEnv("AWS_RESOURCE_CREATION_ENABLED", "yes") == "yes" {
			// role does not exist, create it
			controllerLogger.Debugf("Role does not exist, creating: ecs-%v", serviceName)
			iamRoleArn, err = iam.CreateRole("ecs-"+serviceName, iam.GetEcsTaskIAMTrust())
			if err != nil {
				return nil, err
			}
			// optionally add a policy
			ps := ecs.Paramstore{}
			if ps.IsEnabled() {
				namespace := d.EnvNamespace
				if namespace == "" {
					namespace = serviceName
				}
				controllerLogger.Debugf("Paramstore enabled, putting role: paramstore-%v", namespace)
				err = iam.PutRolePolicy("ecs-"+serviceName, "paramstore-"+namespace, ps.GetParamstoreIAMPolicy(namespace))
				if err != nil {
					return nil, err
				}
			}
		} else {
			return nil, errors.New("IAM Task Role not found and resource creation is disabled")
		}
	} else if err != nil {
		return nil, err
	}

	// retrieving secrets
	secrets := make(map[string]string)
	if util.GetEnv("PARAMSTORE_INJECT", "no") == "yes" {
		ps := ecs.Paramstore{}
		if ps.IsEnabled() {
			err := ps.GetParameters("/"+util.GetEnv("PARAMSTORE_PREFIX", "")+"-"+util.GetEnv("AWS_ACCOUNT_ENV", "")+"/"+serviceName+"/", false)
			if err != nil {
				return nil, err
			}
			for _, v := range ps.Parameters {
				keyName := strings.Split(v.Name, "/")
				secrets[keyName[len(keyName)-1]] = v.Arn
			}
		}
	}

	// create task definition
	e := ecs.ECS{ServiceName: serviceName, IamRoleArn: *iamRoleArn, ClusterName: d.Cluster}
	taskDefArn, err := e.CreateTaskDefinition(d, secrets)
	if err != nil {
		controllerLogger.Errorf("Could not create task def %v", serviceName)
		return nil, err
	}
	controllerLogger.Debugf("Created task definition: %v", *taskDefArn)

	// update service with new task (update desired instance in case of difference)
	controllerLogger.Debugf("Updating service: %v with taskdefarn: %v", serviceName, *taskDefArn)
	serviceExists, err := e.ServiceExists(serviceName)
	if err == nil && !serviceExists {
		controllerLogger.Debugf("service (%v) not found, creating...", serviceName)
		if util.GetEnv("AWS_RESOURCE_CREATION_ENABLED", "yes") == "yes" {
			s.Listeners, err = c.createService(serviceName, d, taskDefArn)
			if err != nil {
				controllerLogger.Errorf("Could not create service %v: %s", serviceName, err)
				return nil, err
			}
			// create service in dynamodb
			err = c.checkAndCreateServiceInDynamo(s, d)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, errors.New("ECS Service not found and resource creation is disabled")
		}
	} else if err != nil {
		return nil, errors.New("Error during checking whether service exists")
	} else {
		// create service in dynamodb
		err = c.checkAndCreateServiceInDynamo(s, d)
		if err != nil {
			return nil, err
		}
		err = c.updateDeployment(d, ddLast, serviceName, taskDefArn, iamRoleArn)
		if err != nil {
			controllerLogger.Errorf("updateDeployment failed: %s", err)
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
	var notification integrations.Notification
	if util.GetEnv("SLACK_WEBHOOKS", "") != "" {
		notification = integrations.NewSlack()
	} else {
		notification = integrations.NewDummy()
	}
	go e.LaunchWaitUntilServicesStable(dd, ddLast, notification)

	ret := &service.DeployResult{
		ServiceName:       serviceName,
		ClusterName:       d.Cluster,
		TaskDefinitionArn: *taskDefArn,
		DeploymentTime:    dd.Time,
	}
	return ret, nil
}

func (c *Controller) updateDeployment(d service.Deploy, ddLast *service.DynamoDeployment, serviceName string, taskDefArn *string, iamRoleArn *string) error {
	s := service.NewService()
	s.ServiceName = serviceName
	s.ClusterName = d.Cluster
	e := ecs.ECS{ServiceName: serviceName, IamRoleArn: *iamRoleArn, ClusterName: d.Cluster, TaskDefArn: taskDefArn}
	updateECSService := true
	// compare with previous deployment if there is one
	if ddLast != nil {
		var err error
		if strings.ToLower(d.ServiceProtocol) != "none" {
			var alb *ecs.ALB
			if d.LoadBalancer == "" {
				alb, err = ecs.NewALB(d.Cluster)
			} else {
				alb, err = ecs.NewALB(d.LoadBalancer)
			}
			targetGroupArn, err := alb.GetTargetGroupArn(serviceName)
			if err != nil {
				return err
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
					return err
				}
			}
			// update loadbalancer if changed
			var noLBChange bool
			if ddLast.DeployData.LoadBalancer == "" && strings.ToLower(d.LoadBalancer) == strings.ToLower(d.Cluster) {
				noLBChange = true
			}
			if strings.ToLower(d.LoadBalancer) != strings.ToLower(ddLast.DeployData.LoadBalancer) && !noLBChange && strings.ToLower(d.ServiceProtocol) != "none" {
				controllerLogger.Infof("LoadBalancer change detected for service %s", serviceName)
				// delete old loadbalancer rules
				var oldAlb *ecs.ALB
				if ddLast.DeployData.LoadBalancer == "" {
					oldAlb, err = ecs.NewALB(ddLast.DeployData.Cluster)
				} else {
					oldAlb, err = ecs.NewALB(ddLast.DeployData.LoadBalancer)
				}
				err = c.deleteRulesForTarget(serviceName, d, targetGroupArn, oldAlb)
				if err != nil {

				}
				// delete target group
				controllerLogger.Debugf("Deleting target group for service: %v", serviceName)
				err = oldAlb.DeleteTargetGroup(*targetGroupArn)
				if err != nil {
					return err
				}
				// create new target group
				controllerLogger.Debugf("Creating target group for service: %v", serviceName)
				newTargetGroupArn, err := alb.CreateTargetGroup(serviceName, d)
				if err != nil {
					return err
				}
				// modify target group attributes
				if d.DeregistrationDelay != -1 || d.Stickiness.Enabled {
					err = alb.ModifyTargetGroupAttributes(*newTargetGroupArn, d)
					if err != nil {
						return err
					}
				}
				// create new rules
				listeners, err := c.createRulesForTarget(serviceName, d, newTargetGroupArn, alb)
				s.Listeners = listeners
				if err != nil {
					return err
				}
				// recreating ecs service
				controllerLogger.Infof("Recreating ecs service: %v", serviceName)
				err = e.DeleteService(d.Cluster, serviceName)
				if err != nil {
					return err
				}
				err = e.WaitUntilServicesInactive(d.Cluster, serviceName)
				if err != nil {
					return err
				}
				// create ecs service
				e.TargetGroupArn = newTargetGroupArn
				err = e.CreateService(d)
				if err != nil {
					return err
				}
				// update listeners
				s.UpdateServiceListeners(s.ClusterName, s.ServiceName, listeners)
				// don't update ecs service later
				updateECSService = false
			} else {
				// check for rules changes
				if c.rulesChanged(d, ddLast) {
					controllerLogger.Infof("Recreating alb rules for: " + serviceName)
					// recreate rules
					err = c.deleteRulesForTarget(serviceName, d, targetGroupArn, alb)
					if err != nil {
						controllerLogger.Infof("Couldn't delete existing rules for target: " + serviceName)
					}
					// create new rules
					_, err := c.createRulesForTarget(serviceName, d, targetGroupArn, alb)
					if err != nil {
						return err
					}
				}
			}
		}
		ps := ecs.Paramstore{}
		if ps.IsEnabled() {
			iam := ecs.IAM{}
			thisNamespace, lastNamespace := d.EnvNamespace, ddLast.DeployData.EnvNamespace
			if thisNamespace == "" {
				thisNamespace = serviceName
			}
			if lastNamespace == "" {
				lastNamespace = serviceName
			}
			if thisNamespace != lastNamespace {
				controllerLogger.Debugf("Paramstore enabled, putting role: paramstore-%v", serviceName)
				err = iam.DeleteRolePolicy("ecs-"+serviceName, "paramstore-"+lastNamespace)
				if err != nil {
					return err
				}
				err = iam.PutRolePolicy("ecs-"+serviceName, "paramstore-"+thisNamespace, ps.GetParamstoreIAMPolicy(thisNamespace))
				if err != nil {
					return err
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
	if updateECSService {
		var err error
		_, err = e.UpdateService(serviceName, taskDefArn, d)
		controllerLogger.Debugf("Updating ecs service: %v", serviceName)
		if err != nil {
			controllerLogger.Errorf("Could not update service %v: %v", serviceName, err)
			return err
		}
	}
	return nil
}

func (c *Controller) rulesChanged(d service.Deploy, ddLast *service.DynamoDeployment) bool {
	if len(d.RuleConditions) != len(ddLast.DeployData.RuleConditions) {
		return true
	}

	// sort rule conditions
	sortedRuleCondition := d.RuleConditions
	ddLastSortedRuleCondition := ddLast.DeployData.RuleConditions
	sort.Sort(ruleConditionSort(sortedRuleCondition))
	sort.Sort(ruleConditionSort(ddLastSortedRuleCondition))
	// loop over rule conditions to compare them
	for k, v := range sortedRuleCondition {
		v2 := ddLastSortedRuleCondition[k]
		if !cmp.Equal(v, v2) {
			return true
		}
	}

	return false

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
func (c *Controller) createService(serviceName string, d service.Deploy, taskDefArn *string) ([]string, error) {
	iam := ecs.IAM{}
	var targetGroupArn *string
	var listeners []string
	var alb *ecs.ALB
	var err error
	if d.LoadBalancer != "" {
		alb, err = ecs.NewALB(d.LoadBalancer)
	} else {
		alb, err = ecs.NewALB(d.Cluster)
	}
	if err != nil {
		return nil, err
	}

	// create target group
	if strings.ToLower(d.ServiceProtocol) != "none" {
		var err error
		controllerLogger.Debugf("Creating target group for service: %v", serviceName)
		targetGroupArn, err = alb.CreateTargetGroup(serviceName, d)
		if err != nil {
			return nil, err
		}
		// modify target group attributes
		if d.DeregistrationDelay != -1 || d.Stickiness.Enabled {
			err = alb.ModifyTargetGroupAttributes(*targetGroupArn, d)
			if err != nil {
				return nil, err
			}
		}

		// deploy rules for target group
		listeners, err = c.createRulesForTarget(serviceName, d, targetGroupArn, alb)
		if err != nil {
			return nil, err
		}
	}

	// check whether ecs-service-role exists
	controllerLogger.Debugf("Checking whether role exists: %v", util.GetEnv("AWS_ECS_SERVICE_ROLE", "ecs-service-role"))
	iamServiceRoleArn, err := iam.RoleExists(util.GetEnv("AWS_ECS_SERVICE_ROLE", "ecs-service-role"))
	if err == nil && iamServiceRoleArn == nil {
		controllerLogger.Debugf("Creating ecs service role")
		_, err = iam.CreateRole(util.GetEnv("AWS_ECS_SERVICE_ROLE", "ecs-service-role"), iam.GetEcsServiceIAMTrust())
		if err != nil {
			return nil, err
		}
		controllerLogger.Debugf("Attaching ecs service role")
		err = iam.AttachRolePolicy(util.GetEnv("AWS_ECS_SERVICE_ROLE", "ecs-service-role"), iam.GetEcsServicePolicy())
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, errors.New("Error during checking whether ecs service role exists")
	}

	// create ecs service
	controllerLogger.Debugf("Creating ecs service: %v", serviceName)
	e := ecs.ECS{ServiceName: serviceName, TaskDefArn: taskDefArn, TargetGroupArn: targetGroupArn}
	err = e.CreateService(d)
	if err != nil {
		return nil, err
	}
	return listeners, nil
}
func (c *Controller) checkAndCreateServiceInDynamo(s *service.Service, d service.Deploy) error {
	serviceExistsInDynamo, err := s.ServiceExistsInDynamo()
	if err == nil && !serviceExistsInDynamo {
		err = c.createServiceInDynamo(s, d)
		if err != nil {
			controllerLogger.Errorf("Could not create service %v in dynamodb", s.ServiceName)
			return err
		}
	}
	return nil
}

func (c *Controller) createServiceInDynamo(s *service.Service, d service.Deploy) error {
	var err error
	e := ecs.ECS{ServiceName: s.ServiceName}

	dsEl := &service.DynamoServicesElement{S: s.ServiceName, C: s.ClusterName, Listeners: s.Listeners}
	dsEl.CpuReservation, dsEl.CpuLimit, dsEl.MemoryReservation, dsEl.MemoryLimit = e.GetContainerLimits(d)

	err = s.CreateService(dsEl)
	if err != nil {
		controllerLogger.Errorf("Could not create/update service (%v) in db: %v", s.ServiceName, err)
		return err
	}
	return nil
}

// Deploy rules for a specific targetGroup
func (c *Controller) deleteRulesForTarget(serviceName string, d service.Deploy, targetGroupArn *string, alb *ecs.ALB) error {
	err := alb.GetRulesForAllListeners()
	if err != nil {
		return err
	}
	ruleArnsToDelete := alb.GetRulesByTargetGroupArn(*targetGroupArn)
	authRuleArns := alb.GetRuleByTargetGroupArnWithAuth(*targetGroupArn)
	for _, authRuleArn := range authRuleArns {
		conditionField, conditionValue := alb.GetConditionsForRule(authRuleArn)
		controllerLogger.Debugf("deleteRulesForTarget: found authRule with conditionField %s and conditionValue %s", strings.Join(conditionField, ","), strings.Join(conditionValue, ","))
		httpListener := alb.GetListenerArnForProtocol("http")
		if httpListener != "" {
			ruleArn, _, err := alb.FindRule(httpListener, "", conditionField, conditionValue)
			if err != nil {
				controllerLogger.Debugf("deleteRulesForTarget: rule not found: %s", err)
			}
			if ruleArn != nil {
				ruleArnsToDelete = append(ruleArnsToDelete, *ruleArn)
			}

		}
	}
	for _, ruleArn := range ruleArnsToDelete {
		alb.DeleteRule(ruleArn)
	}
	return nil
}

// delete rule for a targetgroup with specific listener
func (c *Controller) deleteRuleForTargetWithListener(serviceName string, r *service.DeployRuleConditions, targetGroupArn *string, alb *ecs.ALB, listener string) error {
	_, conditionField, conditionValue := c.getALBConditionFieldAndValue(*r, alb.GetDomain())
	err := alb.GetRulesForAllListeners()
	if err != nil {
		return err
	}
	ruleArn, _, err := alb.FindRule(listener, *targetGroupArn, conditionField, conditionValue)
	if err != nil {
		return err
	}
	return alb.DeleteRule(*ruleArn)
}

// Update rule for a specific targetGroups
func (c *Controller) UpdateRuleForTarget(serviceName string, r *service.DeployRuleConditions, rLast *service.DeployRuleConditions, targetGroupArn *string, alb *ecs.ALB, listener string) error {
	_, conditionField, conditionValue := c.getALBConditionFieldAndValue(*rLast, alb.GetDomain())
	err := alb.GetRulesForAllListeners()
	if err != nil {
		return err
	}
	ruleArn, _, err := alb.FindRule(alb.GetListenerArnForProtocol(listener), *targetGroupArn, conditionField, conditionValue)
	if err != nil {
		return err
	}
	ruleType, _, conditionValue := c.getALBConditionFieldAndValue(*rLast, alb.GetDomain())

	// if cognito is set, a redirect is needed instead (cognito doesn't work with http)
	if strings.ToLower(listener) == "http" && r.CognitoAuth.ClientName != "" {
		return alb.UpdateRuleToHTTPSRedirect(*targetGroupArn, *ruleArn, ruleType, conditionValue)
	}

	return alb.UpdateRule(*targetGroupArn, *ruleArn, ruleType, conditionValue, r.CognitoAuth)

}

func (c *Controller) getALBConditionFieldAndValue(r service.DeployRuleConditions, domain string) (string, []string, []string) {
	if r.PathPattern != "" && r.Hostname != "" {
		return "combined", []string{"path-pattern", "host-header"}, []string{r.PathPattern, r.Hostname + "." + domain}
	}
	if r.PathPattern != "" {
		return "pathPattern", []string{"path-pattern"}, []string{r.PathPattern}
	}
	if r.Hostname != "" {
		return "hostname", []string{"host-header"}, []string{r.Hostname + "." + domain}
	}
	return "", []string{}, []string{}
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
		ruleConditionsSorted := d.RuleConditions
		sort.Sort(ruleConditionSort(ruleConditionsSorted))
		for _, r := range ruleConditionsSorted {
			ruleType, _, conditionValue := c.getALBConditionFieldAndValue(*r, alb.GetDomain())
			l, err := alb.CreateRuleForListeners(ruleType, r.Listeners, *targetGroupArn, conditionValue, (priority + 10 + int64(newRules)), r.CognitoAuth)
			if err != nil {
				return nil, err
			}
			newRules += len(r.Listeners)
			listeners = append(listeners, l...)
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

	sort.Slice(rss, func(i, j int) bool {
		if strings.Compare(rss[i].ServiceName, rss[j].ServiceName) == 1 {
			return false
		}
		return true
	})

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

func (c *Controller) runTask(serviceName string, runTask service.RunTask) (string, error) {
	s := service.NewService()
	s.ServiceName = serviceName
	var taskArn string
	clusterName, err := s.GetClusterName()
	if err != nil {
		return taskArn, err
	}
	dd, err := s.GetLastDeploy()
	if err != nil {
		return taskArn, err
	}
	e := ecs.ECS{}
	taskDefinition, err := e.GetTaskDefinition(clusterName, serviceName)
	if err != nil {
		return taskArn, err
	}
	taskArn, err = e.RunTask(clusterName, taskDefinition, runTask, *dd.DeployData)
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
		if err.Error() == "dynamo: no item found" {
			controllerLogger.Infof("Database is empty - starting app for the first time")
			err = s.InitDB(apiVersion)
			if err != nil {
				return err
			}
		} else {
			return err
		}
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
			var ddLast service.DynamoDeployment
			if i == 0 {
				ddLast = dds[i]
			} else {
				ddLast = dds[i-1]
			}
			var notification integrations.Notification
			if util.GetEnv("SLACK_WEBHOOKS", "") != "" {
				notification = integrations.NewSlack()
			} else {
				notification = integrations.NewDummy()
			}
			go e.LaunchWaitUntilServicesStable(&dds[i], &ddLast, notification)

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
			// get cluster info
			ciArns, err := e.ListContainerInstances(clusterName)
			if err != nil {
				return err
			}
			cis, err := e.DescribeContainerInstances(clusterName, ciArns)
			if err != nil {
				return err
			}
			// check for lifecycle hook
			var lifecycleHookNotFound bool
			hn, err := autoscaling.GetLifecycleHookNames(autoScalingGroupName, "autoscaling:EC2_INSTANCE_TERMINATING")
			if err != nil || len(hn) == 0 {
				controllerLogger.Errorf("Cluster %v doesn't have a lifecycle hook", clusterName)
				lifecycleHookNotFound = true
			}
			if !lifecycleHookNotFound {
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
							s.PutClusterInfo(*dc, clusterName, "no", "")
						}
						// launch wait for drained
						controllerLogger.Infof("Launching waitForDrainedNode for cluster=%v, instance=%v, autoscalingGroupName=%v", clusterName, ci.Ec2InstanceId, autoScalingGroupName)
						go e.LaunchWaitForDrainedNode(clusterName, ci.ContainerInstanceArn, ci.Ec2InstanceId, autoScalingGroupName, hn[0], "")
					}
				}
			}
			// TODO: check for pending autoscaling actions
			if len(cis) == 0 {
				return errors.New("Couldn't retrieve any EC2 Container instances")
			}
			f, err := e.ConvertResourceToRir(cis[0].RegisteredResources)
			if err != nil {
				return err
			}
			asc := AutoscalingController{}
			registeredInstanceCpu := f.RegisteredCpu
			registeredInstanceMemory := f.RegisteredMemory
			for _, scalingOp := range []string{"up", "down"} {
				period, interval := asc.getAutoscalingPeriodInterval(scalingOp)
				startTime := time.Now().Add(-1 * time.Duration(period) * time.Duration(interval) * time.Second)
				_, pendingAction, err := s.GetScalingActivity(clusterName, startTime)
				if err != nil {
					return err
				}
				if pendingAction == scalingOp {
					controllerLogger.Infof("Launching process for pending scaling operation: %s ", pendingAction)
					go asc.launchProcessPendingScalingOp(clusterName, pendingAction, registeredInstanceCpu, registeredInstanceMemory)
				}
			}
		}
	}
	// Start autoscaling polling if enabled
	autoscalingStrategies := strings.Split(util.GetEnv("AUTOSCALING_STRATEGIES", ""), ",")
	for _, v := range autoscalingStrategies {
		if strings.ToLower(v) == "polling" {
			asc := AutoscalingController{}
			controllerLogger.Debugf("Starting AutoscalingPollingStrategy in goroutine")
			go asc.startAutoscalingPollingStrategy()
		}
	}
	controllerLogger.Debugf("Finished controller resume. Checked %d services", len(dds))
	return err
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
	if len(b.LoadBalancers) == 0 {
		b.LoadBalancers = []service.LoadBalancer{
			{
				Name:          b.ClusterName,
				IPAddressType: "ipv4",
				Scheme:        "internet-facing",
				Type:          "application",
			},
		}
	}
	var albs []*ecs.ALB
	// create load balancer, default target, and listener
	for _, v := range b.LoadBalancers {
		alb, err := ecs.NewALBAndCreate(v.Name, v.IPAddressType, v.Scheme, strings.Split(b.AlbSecurityGroups, ","), strings.Split(b.EcsSubnets, ","), v.Type)
		if err != nil {
			return err
		}
		defaultTargetGroupArn, err := alb.CreateTargetGroup(v.Name+"-ecs-deploy", ecsDeploy /* ecs deploy object */)
		if err != nil {
			return err
		}
		err = alb.CreateListener("HTTP", 80, *defaultTargetGroupArn)
		if err != nil {
			return err
		}
		albs = append(albs, alb)
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
	for _, alb := range albs {
		fmt.Printf("     URL: http://%v/ecs-deploy                  \n", alb.DnsName)
	}
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
	if len(b.LoadBalancers) == 0 {
		b.LoadBalancers = []service.LoadBalancer{
			{
				Name:          b.ClusterName,
				IPAddressType: "ipv4",
				Scheme:        "internet-facing",
				Type:          "application",
			},
		}
	}
	for _, v := range b.LoadBalancers {
		alb, err := ecs.NewALB(v.Name)
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
				alb.DeleteTargetGroup(*targetGroup)
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

func (c *Controller) putServiceAutoscaling(serviceName string, autoscaling service.Autoscaling) (string, error) {
	var result string
	var writeChanges bool
	// validation
	if autoscaling.MinimumCount == 0 && autoscaling.MaximumCount == 0 {
		return result, errors.New("minimumCount / maximumCount missing")
	}
	// autoscaling
	as := ecs.AutoScaling{}
	cloudwatch := ecs.CloudWatch{}
	iam := ecs.IAM{}
	s := service.NewService()
	s.ServiceName = serviceName
	clusterName, err := s.GetClusterName()
	if err != nil {
		return result, err
	}
	dd, err := s.GetLastDeploy()
	if err != nil {
		return result, err
	}
	resourceId := "service/" + clusterName + "/" + serviceName
	// check whether iam role exists
	var autoscalingRoleArn *string
	autoscalingRoleName := "ecs-app-autoscaling-role"
	autoscalingRoleArn, err = iam.RoleExists(autoscalingRoleName)
	if err != nil {
		return result, err
	}
	if err == nil && autoscalingRoleArn == nil {
		autoscalingRoleArn, err = iam.CreateRole(autoscalingRoleName, iam.GetEcsAppAutoscalingIAMTrust())
		if err != nil {
			return result, err
		}
		err = iam.AttachRolePolicy(autoscalingRoleName, "arn:aws:iam::aws:policy/service-role/AmazonEC2ContainerServiceAutoscaleRole")
		if err != nil {
			return result, err
		}
	}
	// register scalable target
	if dd.Scaling.Autoscaling.ResourceId == "" {
		err = as.RegisterScalableTarget(autoscaling.MinimumCount, autoscaling.MaximumCount, resourceId, *autoscalingRoleArn)
		if err != nil {
			return result, err
		}
	} else {
		// describe -> check difference, apply difference
		a, err := as.DescribeScalableTargets([]string{resourceId})
		if err != nil {
			return result, err
		}
		if len(a) == 0 {
			return result, errors.New("Couldn't describe scalable target")
		}
		if a[0].MinimumCount != autoscaling.MinimumCount || a[0].MaximumCount != autoscaling.MaximumCount {
			err = as.RegisterScalableTarget(autoscaling.MinimumCount, autoscaling.MaximumCount, resourceId, *autoscalingRoleArn)
			if err != nil {
				return result, err
			}
		}
	}
	// change desired count if necessary
	if dd.Scaling.DesiredCount != autoscaling.DesiredCount {
		e := ecs.ECS{}
		e.ManualScaleService(clusterName, serviceName, autoscaling.DesiredCount)
		writeChanges = true
	}
	// Add Autoscaling policy
	if len(autoscaling.Policies) > 0 {
		for _, p := range autoscaling.Policies {
			// autoscaling up or down?
			var autoscalingType string
			if p.ScalingAdjustment > 0 {
				autoscalingType = "up"
			} else {
				autoscalingType = "down"
			}
			// metric name
			var metricName, metricNamespace string
			if p.Metric == "cpu" {
				metricName = "CPUUtilization"
				metricNamespace = "AWS/ECS"
			} else {
				metricName = "MemoryUtilization"
				metricNamespace = "AWS/ECS"
			}
			// autoscaling policies
			autoscalingPolicyArns := make(map[string]struct{})
			for _, v := range dd.Scaling.Autoscaling.PolicyNames {
				autoscalingPolicyArns[v] = struct{}{}
			}
			// set policy name
			policyName := serviceName + "-" + p.Metric + "-" + autoscalingType
			if _, exists := autoscalingPolicyArns[policyName]; !exists {
				// put scaling policy
				scalingPolicyArn, err := as.PutScalingPolicy(policyName, resourceId, 300, p.ScalingAdjustment)
				if err != nil {
					return result, err
				}
				// put metric alarm
				err = cloudwatch.PutMetricAlarm(serviceName, clusterName, policyName, []string{scalingPolicyArn}, policyName, p.DatapointsToAlarm, metricName, metricNamespace, p.Period, p.Threshold, strings.Title(p.ComparisonOperator), strings.Title(p.ThresholdStatistic), p.EvaluationPeriods)
				if err != nil {
					return result, err
				}
				// write changes to the database
				writeChanges = true
				dd.Scaling.Autoscaling.PolicyNames = append(dd.Scaling.Autoscaling.PolicyNames, policyName)
			}
		}
	}

	if writeChanges {
		err = s.SetAutoscalingProperties(autoscaling.DesiredCount, resourceId, dd.Scaling.Autoscaling.PolicyNames)
		if err != nil {
			return result, err
		}
	}

	return "OK", nil
}
func (c *Controller) getServiceAutoscaling(serviceName string) (service.Autoscaling, error) {
	var a service.Autoscaling
	e := ecs.ECS{}
	s := service.NewService()
	s.ServiceName = serviceName
	clusterName, err := s.GetClusterName()
	autoscaling := ecs.AutoScaling{}
	cloudwatch := ecs.CloudWatch{}

	// get last deploy
	dd, err := s.GetLastDeploy()
	if err != nil {
		return a, err
	}

	if dd.Scaling.Autoscaling.ResourceId == "" {
		return a, nil
	}

	// get min, max capacity
	as, err := autoscaling.DescribeScalableTargets([]string{dd.Scaling.Autoscaling.ResourceId})
	if err != nil {
		return a, err
	}
	if len(as) == 0 {
		return a, errors.New("No scalable target returned")
	}
	a = as[0]

	// get desiredCount
	runningService, err := e.DescribeService(clusterName, serviceName, false, false, false)
	if err != nil {
		return a, err
	}
	a.DesiredCount = runningService.DesiredCount

	// get policy
	apsPolicy, err := autoscaling.DescribeScalingPolicies(dd.Scaling.Autoscaling.PolicyNames, dd.Scaling.Autoscaling.ResourceId)
	if err != nil {
		return a, err
	}
	// get alarm
	aps, err := cloudwatch.DescribeAlarms(dd.Scaling.Autoscaling.PolicyNames)
	if err != nil {
		return a, err
	}

	for k, v := range aps {
		for _, v2 := range apsPolicy {
			if v.PolicyName == v2.PolicyName {
				aps[k].ScalingAdjustment = v2.ScalingAdjustment
			}
		}
	}

	a.Policies = aps

	return a, nil
}
func (c *Controller) deleteServiceAutoscalingPolicy(serviceName, policyName string) error {
	s := service.NewService()
	s.ServiceName = serviceName
	autoscaling := ecs.AutoScaling{}
	cloudwatch := ecs.CloudWatch{}

	// get last deploy
	dd, err := s.GetLastDeploy()
	if err != nil {
		return err
	}

	if dd.Scaling.Autoscaling.ResourceId == "" {
		return errors.New("Autoscaling not active for service")
	}

	var newPolicyNames []string
	var found bool
	for _, v := range dd.Scaling.Autoscaling.PolicyNames {
		if v == policyName {
			found = true
		} else {
			newPolicyNames = append(newPolicyNames, v)
		}
	}
	if !found {
		return fmt.Errorf("Autoscaling policy %v not found", policyName)
	}

	err = autoscaling.DeleteScalingPolicy(policyName, dd.Scaling.Autoscaling.ResourceId)
	if err != nil {
		return err
	}

	err = cloudwatch.DeleteAlarms([]string{policyName})
	if err != nil {
		return err
	}

	// write changes to db
	err = s.SetAutoscalingProperties(dd.Scaling.DesiredCount, dd.Scaling.Autoscaling.ResourceId, newPolicyNames)
	if err != nil {
		return err
	}

	return nil
}

func (c *Controller) deleteServiceAutoscaling(serviceName string) error {
	s := service.NewService()
	s.ServiceName = serviceName
	autoscaling := ecs.AutoScaling{}
	cloudwatch := ecs.CloudWatch{}

	// get last deploy
	dd, err := s.GetLastDeploy()
	if err != nil {
		return err
	}

	if dd.Scaling.Autoscaling.ResourceId == "" {
		return errors.New("Autoscaling not active for service")
	}

	for _, policyName := range dd.Scaling.Autoscaling.PolicyNames {
		err = autoscaling.DeleteScalingPolicy(policyName, dd.Scaling.Autoscaling.ResourceId)
		if err != nil {
			return err
		}

		err = cloudwatch.DeleteAlarms([]string{policyName})
		if err != nil {
			return err
		}
	}

	err = autoscaling.DeregisterScalableTarget(dd.Scaling.Autoscaling.ResourceId)
	if err != nil {
		return err
	}

	// write changes to db
	err = s.SetAutoscalingProperties(dd.Scaling.DesiredCount, "", []string{})
	if err != nil {
		return err
	}

	return nil
}
