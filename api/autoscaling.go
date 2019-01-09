package api

import (
	"github.com/in4it/ecs-deploy/provider/ecs"
	"github.com/in4it/ecs-deploy/service"
	"github.com/in4it/ecs-deploy/util"
	"github.com/juju/loggo"

	"encoding/json"
	"errors"
	"io/ioutil"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type AutoscalingController struct {
}

var asAutoscalingControllerLogger = loggo.GetLogger("as-controller")

func (c *AutoscalingController) getClusterInfoWithCache(clusterName string) (*service.DynamoCluster, error) {
	return c.getClusterInfo(clusterName, true)
}
func (c *AutoscalingController) getClusterInfo(clusterName string, withCache bool) (*service.DynamoCluster, error) {
	s := service.NewService()
	e := ecs.ECS{}

	var dc *service.DynamoCluster
	var err error

	if withCache {
		dc, err = s.GetClusterInfo()
		if err != nil {
			return nil, err
		}
	}
	if dc == nil || dc.Time.Before(time.Now().Add(-4*time.Minute /* 4 minutes cache */)) {
		// no cache, need to retrieve everything
		asAutoscalingControllerLogger.Debugf("No cache found, need to retrieve using API calls")
		dc = &service.DynamoCluster{}
		// calculate free resources
		firs, _, err := e.GetInstanceResources(clusterName)
		if err != nil {
			return nil, err
		}
		for _, f := range firs {
			var dcci service.DynamoClusterContainerInstance
			dcci.ClusterName = clusterName
			dcci.ContainerInstanceId = f.InstanceId
			dcci.AvailabilityZone = f.AvailabilityZone
			dcci.FreeMemory = f.FreeMemory
			dcci.FreeCpu = f.FreeCpu
			dcci.Status = f.Status
			dc.ContainerInstances = append(dc.ContainerInstances, dcci)
		}
	}
	return dc, nil
}

// return minimal cpu/memory resources that are needed for the cluster
func (c *AutoscalingController) getResourcesNeeded(clusterName string) (int64, int64, error) {
	cc := Controller{}
	dss, _ := cc.getServices()
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
		return 0, 0, errors.New("Minimal Memory needed for clusterName " + clusterName + " not found")
	}
	if _, ok := cpuNeeded[clusterName]; !ok {
		return 0, 0, errors.New("Minimal CPU needed for clusterName " + clusterName + " not found")
	}
	return memoryNeeded[clusterName], cpuNeeded[clusterName], nil
}
func (c *AutoscalingController) getAutoscalingStrategy() (bool, bool) {
	// Check whether Strategy is enabled
	asStrategies := strings.Split(util.GetEnv("AUTOSCALING_STRATEGIES", "LargestContainerUp,LargestContainerDown"), ",")
	asStrategyLargestContainerUp := false
	asStrategyLargestContainerDown := false
	for _, v := range asStrategies {
		if strings.ToLower(v) == "largestcontainerup" {
			asStrategyLargestContainerUp = true
		}
		if strings.ToLower(v) == "largestcontainerdown" {
			asStrategyLargestContainerDown = true
		}
	}
	return asStrategyLargestContainerUp, asStrategyLargestContainerDown
}

// Process ECS event message and determine to scale or not
func (c *AutoscalingController) processEcsMessage(message ecs.SNSPayloadEcs) error {
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
	memoryNeeded, cpuNeeded, err := c.getResourcesNeeded(clusterName)
	if err != nil {
		return err
	}
	// calculate registered resources of the EC2 instance
	f, err := e.ConvertResourceToRir(message.Detail.RegisteredResources)
	if err != nil {
		return err
	}
	registeredInstanceCpu := f.RegisteredCpu
	registeredInstanceMemory := f.RegisteredMemory
	// determine minimum reservations
	dc, err := c.getClusterInfoWithCache(clusterName)
	if err != nil {
		return err
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
	// Check whether Strategy is enabled
	asStrategyLargestContainerUp, asStrategyLargestContainerDown := c.getAutoscalingStrategy()
	// make scaling (up) decision
	var resourcesFitGlobal bool
	var scalingOp = "no"
	var pendingScalingOp string
	if asStrategyLargestContainerUp {
		if desiredCapacity < maxSize {
			resourcesFitGlobal = c.scaleUpDecision(clusterName, dc.ContainerInstances, cpuNeeded, memoryNeeded)
			if !resourcesFitGlobal {
				cooldownMin, err := strconv.ParseInt(util.GetEnv("AUTOSCALING_UP_COOLDOWN", "5"), 10, 64)
				if err != nil {
					cooldownMin = 5
				}
				startTime := time.Now().Add(-1 * time.Duration(cooldownMin) * time.Minute)
				lastScalingOp, _, err := s.GetScalingActivity(clusterName, startTime)
				if err != nil {
					return err
				}
				if lastScalingOp == "no" {
					if util.GetEnv("AUTOSCALING_UP_STRATEGY", "immediately") == "gracefully" {
						pendingScalingOp = "up"
					} else {
						asAutoscalingControllerLogger.Infof("Initiating scaling activity")
						scalingOp = "up"
						err = autoscaling.ScaleClusterNodes(autoScalingGroupName, 1)
						if err != nil {
							return err
						}
					}
				}
			}
		}
	} else {
		// if strategy is "latgestContainerUp" is disabled, resources always fit, and scaling down always needs to be checked
		resourcesFitGlobal = true
	}
	// make scaling (down) decision
	if asStrategyLargestContainerDown && desiredCapacity > minSize && (resourcesFitGlobal || desiredCapacity == maxSize) {
		hasFreeResourcesGlobal := c.scaleDownDecision(clusterName, dc.ContainerInstances, registeredInstanceCpu, registeredInstanceMemory, cpuNeeded, memoryNeeded)
		if hasFreeResourcesGlobal {
			// check cooldown period
			cooldownMin, err := strconv.ParseInt(util.GetEnv("AUTOSCALING_DOWN_COOLDOWN", "5"), 10, 64)
			if err != nil {
				cooldownMin = 5
			}
			startTime := time.Now().Add(-1 * time.Duration(cooldownMin) * time.Minute)
			lastScalingOp, tmpPendingScalingOp, err := s.GetScalingActivity(clusterName, startTime)
			if err != nil {
				return err
			}
			// check whether there is a deploy running
			deployRunning, err := s.IsDeployRunning()
			if err != nil {
				return err
			}
			// only scale down if the cooldown period is not active and if there are no deploys currently running
			if lastScalingOp == "no" && tmpPendingScalingOp == "" && !deployRunning {
				pendingScalingOp = "down"
			}
		}
	}
	// write object
	_, err = s.PutClusterInfo(*dc, clusterName, scalingOp, pendingScalingOp)
	if err != nil {
		return err
	}
	if pendingScalingOp != "" {
		asAutoscalingControllerLogger.Infof("Scaling operation: scaling %s pending", pendingScalingOp)
		go c.launchProcessPendingScalingOp(clusterName, pendingScalingOp, registeredInstanceCpu, registeredInstanceMemory)
	}
	return nil
}
func (c *AutoscalingController) getAutoscalingPeriodInterval(scalingOp string) (int64, int64) {
	var period, interval int64
	var err error
	if scalingOp == "down" {
		period, err = strconv.ParseInt(util.GetEnv("AUTOSCALING_DOWN_PERIOD", "5"), 10, 64)
		if err != nil {
			period = 5
		}
		interval, err = strconv.ParseInt(util.GetEnv("AUTOSCALING_DOWN_INTERVAL", "60"), 10, 64)
		if err != nil {
			interval = 60
		}
	} else if scalingOp == "up" {
		period, err = strconv.ParseInt(util.GetEnv("AUTOSCALING_UP_PERIOD", "2"), 10, 64)
		if err != nil {
			period = 5
		}
		interval, err = strconv.ParseInt(util.GetEnv("AUTOSCALING_UP_INTERVAL", "60"), 10, 64)
		if err != nil {
			interval = 60
		}
	} else {
		return 5, 60
	}
	return period, interval
}
func (c *AutoscalingController) launchProcessPendingScalingOp(clusterName, scalingOp string, registeredInstanceCpu, registeredInstanceMemory int64) error {
	var err error
	var dcNew *service.DynamoCluster
	var sizeChange int64
	s := service.NewService()

	if scalingOp == "up" {
		sizeChange = 1
	} else if scalingOp == "down" {
		sizeChange = -1
	} else {
		return errors.New("Scalingop " + scalingOp + " not recognized")
	}

	period, interval := c.getAutoscalingPeriodInterval(scalingOp)

	var abort, deployRunning, hasFreeResourcesGlobal, resourcesFit bool
	var i int64
	for i = 0; i < period && !abort; i++ {
		time.Sleep(time.Duration(interval) * time.Second)
		dcNew, err = c.getClusterInfo(clusterName, true)
		if err != nil {
			return err
		}
		memoryNeeded, cpuNeeded, err := c.getResourcesNeeded(clusterName)
		if err != nil {
			return err
		}
		// pending scaling down logic
		if scalingOp == "down" {
			hasFreeResourcesGlobal = c.scaleDownDecision(clusterName, dcNew.ContainerInstances, registeredInstanceCpu, registeredInstanceMemory, cpuNeeded, memoryNeeded)
			if hasFreeResourcesGlobal {
				deployRunning, err = s.IsDeployRunning()
				if err != nil {
					return err
				}
				if deployRunning {
					abort = true
				}
			} else {
				abort = true
			}
		} else {
			// pendign scaling up logic
			resourcesFit = c.scaleUpDecision(clusterName, dcNew.ContainerInstances, cpuNeeded, memoryNeeded)
			if resourcesFit {
				abort = true
			}
		}
	}

	if !abort {
		asAutoscalingControllerLogger.Infof("Scaling operation: scaling %s now (%d)", scalingOp, sizeChange)
		autoscaling := ecs.AutoScaling{}
		autoScalingGroupName, err := autoscaling.GetAutoScalingGroupByTag(clusterName)
		if err != nil {
			return err
		}
		err = autoscaling.ScaleClusterNodes(autoScalingGroupName, sizeChange)
		if err != nil {
			return err
		}
		_, err = s.PutClusterInfo(*dcNew, clusterName, scalingOp, "")
		if err != nil {
			return err
		}
	} else {
		asAutoscalingControllerLogger.Infof("Scaling operation: scaling %s aborted. deploy running: %v, free resources (scaling down): %v, resources fit (scaling up): %v", scalingOp, deployRunning, hasFreeResourcesGlobal, resourcesFit)
	}
	return nil
}
func (c *AutoscalingController) scaleUpDecision(clusterName string, containerInstances []service.DynamoClusterContainerInstance, cpuNeeded, memoryNeeded int64) bool {
	resourcesFit := make(map[string]bool)
	resourcesFitGlobal := true
	for _, dcci := range containerInstances {
		if clusterName == dcci.ClusterName {
			if dcci.Status != "DRAINING" && dcci.FreeCpu > cpuNeeded && dcci.FreeMemory > memoryNeeded {
				resourcesFit[dcci.AvailabilityZone] = true
				asAutoscalingControllerLogger.Debugf("Cluster %v needs at least %v cpu and %v memory. Found instance %v (%v) with %v cpu and %v memory",
					clusterName,
					cpuNeeded,
					memoryNeeded,
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
	}
	for k, v := range resourcesFit {
		if !v {
			resourcesFitGlobal = false
			asAutoscalingControllerLogger.Infof("No instance found in %v with %v cpu and %v memory free", k, cpuNeeded, memoryNeeded)
		}
	}
	return resourcesFitGlobal
}
func (c *AutoscalingController) scaleDownDecision(clusterName string, containerInstances []service.DynamoClusterContainerInstance, instanceCpu, instanceMemory, cpuNeeded, memoryNeeded int64) bool {
	var clusterMemoryNeeded = instanceMemory + memoryNeeded            // capacity of full container node + biggest task
	clusterMemoryNeeded += int64(math.Ceil(float64(memoryNeeded) / 2)) // + buffer
	var clusterCpuNeeded = instanceCpu + cpuNeeded
	clusterCpuNeeded += int64(math.Ceil(float64(cpuNeeded) / 2)) // + buffer
	totalFreeCpu := make(map[string]int64)
	totalFreeMemory := make(map[string]int64)
	hasFreeResources := make(map[string]bool)
	hasFreeResourcesGlobal := true
	for _, dcci := range containerInstances {
		if clusterName == dcci.ClusterName {
			if dcci.Status != "DRAINING" {
				totalFreeCpu[dcci.AvailabilityZone] += dcci.FreeCpu
				totalFreeMemory[dcci.AvailabilityZone] += dcci.FreeMemory
			}
		}
	}
	asStrategyLargestContainerUp, _ := c.getAutoscalingStrategy()
	if !asStrategyLargestContainerUp { // if we're not using the LargestContainerUp strategy, scale down only when there's a full instance size of extra resources
		clusterMemoryNeeded = instanceMemory
		clusterCpuNeeded = instanceCpu
	}
	for k, _ := range totalFreeCpu {
		asAutoscalingControllerLogger.Debugf("%v: Have %d cpu available, need %d", k, totalFreeCpu[k], clusterCpuNeeded)
		asAutoscalingControllerLogger.Debugf("%v: Have %d memory available, need %d", k, totalFreeMemory[k], clusterMemoryNeeded)
		if totalFreeCpu[k] >= clusterCpuNeeded && totalFreeMemory[k] >= clusterMemoryNeeded {
			hasFreeResources[k] = true
		} else {
			// set hasFreeResources[k] in case the map key hasn't been set to true
			if _, ok := hasFreeResources[k]; !ok {
				hasFreeResources[k] = false
			}
		}
	}
	if asStrategyLargestContainerUp {
		// when using LargestContainerUp, only downscale when all AZs have too much capacity, otherwise a scaleUp will immediately be triggered
		for k, v := range hasFreeResources {
			asAutoscalingControllerLogger.Debugf("%v has free resources: %v", k, v)
			if !v {
				hasFreeResourcesGlobal = false
			}
		}
	} else {
		// when not using the strategy LargestContainerUp, set hasFreeResourcesGlobal to true if any of the nodes has too many resources
		foundTrue := false
		for k, v := range hasFreeResources {
			asAutoscalingControllerLogger.Debugf("%v has free resources: %v", k, v)
			if v {
				foundTrue = true
			}
		}
		if !foundTrue {
			hasFreeResourcesGlobal = false
		}
	}

	return hasFreeResourcesGlobal
}
func (c *AutoscalingController) processLifecycleMessage(message ecs.SNSPayloadLifecycle) error {
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
		s.PutClusterInfo(*dc, clusterName, "no", "")
	}
	// monitor drained node
	go e.LaunchWaitForDrainedNode(clusterName, containerInstanceArn, message.Detail.EC2InstanceId, message.Detail.AutoScalingGroupName, message.Detail.LifecycleHookName, message.Detail.LifecycleActionToken)
	return nil
}

// start autoscaling polling
func (c *AutoscalingController) startAutoscalingPollingStrategy() {
	e := ecs.ECS{}
	s := service.NewService()
	showEvents := true
	showTasks := false
	showStoppedTasks := false
	lastChecked := time.Now().Add(-1 * time.Minute)
	servicesFound := make(map[string]int)
	// init
	err := s.AutoscalingPullInit()
	if err != nil {
		asAutoscalingControllerLogger.Errorf("couldn't initialize autoscalingpull in backend: %v", err)
	}
	localId, err := c.getLocalId()
	if err != nil {
		asAutoscalingControllerLogger.Errorf("Error while setting getting localId: %v", err)
	}
	asAutoscalingControllerLogger.Infof("ecs-deploy local ID: %v", localId)
	for {
		// only 1 process should do the checking, lock row in dynamodb
		lock, err := s.AutoscalingPullAcquireLock(localId)
		if err != nil {
			asAutoscalingControllerLogger.Errorf("Error while setting lock for pullautoscaling: %v", err)
		}
		if lock {
			services := make(map[string][]*string)
			// get services
			var dss service.DynamoServices
			err := s.GetServices(&dss)
			if err != nil {
				asAutoscalingControllerLogger.Errorf("couldn't get services from backend: %v", err)
			}
			// describe services
			for _, ds := range dss.Services {
				services[ds.C] = append(services[ds.C], &ds.S)
			}
			for clusterName, serviceList := range services {
				rss, err := e.DescribeServicesWithOptions(clusterName, serviceList, showEvents, showTasks, showStoppedTasks, map[string]string{"sleep": "1"})
				if err != nil {
					asAutoscalingControllerLogger.Errorf("Error occured during describe services: %v", err)
				}
				for _, rs := range rss {
					if rs.DesiredCount > rs.RunningCount {
						scaled := false
						if servicesFound[clusterName+":"+rs.ServiceName] < 6 {
							servicesFound[clusterName+":"+rs.ServiceName] += 1
						}
						asAutoscalingControllerLogger.Debugf("Checking service %v for unschedulable tasks where desired count > running count (count: %d)", rs.ServiceName, servicesFound[clusterName+":"+rs.ServiceName])
						for _, event := range rs.Events {
							if event.CreatedAt.After(lastChecked) {
								scaled = c.scaleWhenUnschedulableMessage(clusterName, event.Message)
							}
						}
						if len(rs.Events) > 0 && servicesFound[clusterName+":"+rs.ServiceName] == 5 {
							scaled = c.scaleWhenUnschedulableMessage(clusterName, rs.Events[0].Message)
						}
						if scaled {
							servicesFound[clusterName+":"+rs.ServiceName] = 0
							// write record in dynamodb
							dc, err := s.GetClusterInfo()
							if err != nil {
								asAutoscalingControllerLogger.Debugf("Error while doing GetClusterInfo: %v", err)
							}
							_, err = s.PutClusterInfo(*dc, clusterName, "up", "")
							if err != nil {
								asAutoscalingControllerLogger.Debugf("Error while doing PutClusterInfo: %v", err)
							}
						}
					}
				}
			}
			lastChecked = time.Now()
		}
		time.Sleep(60 * time.Second)
	}
}
func (c *AutoscalingController) scaleWhenUnschedulableMessage(clusterName, message string) bool {
	if strings.Contains(message, "was unable to place a task because no container instance met all of its requirements") && strings.Contains(message, "has insufficient") {
		autoscaling := ecs.AutoScaling{}
		asAutoscalingControllerLogger.Infof("Scaling operation: scaling up now")
		autoScalingGroupName, err := autoscaling.GetAutoScalingGroupByTag(clusterName)
		if err != nil {
			asAutoscalingControllerLogger.Errorf("Error: %v", err)
		} else {
			err = autoscaling.ScaleClusterNodes(autoScalingGroupName, 1)
			if err != nil {
				asAutoscalingControllerLogger.Errorf("Error: %v", err)
			}
		}
		return true
	}
	return false
}

func (c *AutoscalingController) getLocalId() (string, error) {
	ret := "ecs-deploy-" + util.RandStringBytesMaskImprSrc(8)
	var task ecs.EcsTaskMetadata
	url := util.GetEnv("ECS_CONTAINER_METADATA_URI", "") + "/task"
	timeout := time.Duration(10 * time.Second)
	client := http.Client{
		Timeout: timeout,
	}
	resp, err := client.Get(url)
	if err != nil {
		return ret, err
	}
	defer resp.Body.Close()
	contents, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return ret, err
	}
	err = json.Unmarshal(contents, &task)
	if err != nil {
		return ret, err
	}
	split := strings.Split(task.TaskARN, "task/")
	if len(split) != 2 {
		return ret, err
	}
	return split[1], nil
}
