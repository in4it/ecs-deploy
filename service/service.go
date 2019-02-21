package service

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/guregu/dynamo"
	"github.com/in4it/ecs-deploy/util"
	"github.com/juju/loggo"

	"errors"
	"math"
	"strings"
	"time"
)

// logging
var serviceLogger = loggo.GetLogger("service")

type Service struct {
	db          *dynamo.DB
	table       dynamo.Table
	ServiceName string
	ClusterName string
	Listeners   []string
}

type DynamoDeployment struct {
	ServiceName       string    `dynamo:"ServiceName,hash"`
	Time              time.Time `dynamo:"Time,range" index:"DayIndex,range" index:"MonthIndex,range"`
	Day               string    `index:"DayIndex,hash"`
	Month             string    `index:"MonthIndex,hash"`
	Status            string
	DeployError       string
	Tag               string
	Scaling           DynamoDeploymentScaling
	ManualTasksArns   []string
	TaskDefinitionArn *string
	DeployData        *Deploy
	Version           int64
}

type DynamoDeploymentScaling struct {
	DesiredCount int64
	Autoscaling  DynamoDeploymentAutoscaling
}

type DynamoDeploymentAutoscaling struct {
	ResourceId  string
	PolicyNames []string
}

// dynamo services struct
type DynamoServices struct {
	ServiceName string `dynamo:"ServiceName,hash"`
	Services    []*DynamoServicesElement
	Time        string `dynamo:"Time,range"`
	Version     int64
	ApiVersion  string
}
type DynamoServicesElement struct {
	C                 string
	S                 string
	MemoryLimit       int64    `dynamo:"ML"`
	MemoryReservation int64    `dynamo:"MR"`
	CpuLimit          int64    `dynamo:"CL"`
	CpuReservation    int64    `dynamo:"CR"`
	Listeners         []string `dynamo:"L"`
}

// dynamo cluster struct
type DynamoCluster struct {
	Identifier         string    `dynamo:"ServiceName,hash"`
	Time               time.Time `dynamo:"Time,range"`
	ContainerInstances []DynamoClusterContainerInstance
	ScalingOperation   DynamoClusterScalingOperation
	ExpirationTime     time.Time
	ExpirationTimeTTL  int64
}
type DynamoClusterScalingOperation struct {
	ClusterName   string
	Action        string
	PendingAction string
}
type DynamoClusterContainerInstance struct {
	ClusterName         string
	ContainerInstanceId string
	AvailabilityZone    string
	FreeMemory          int64
	FreeCpu             int64
	Status              string
}

// dynamo pull struct
type DynamoAutoscalingPull struct {
	Identifier    string    `dynamo:"ServiceName,hash"`
	Time          string    `dynamo:"Time,range"`
	Lock          string    `dynamo:"L"`
	LockTimestamp time.Time `dynamo:"LT"`
}

func NewService() *Service {
	s := Service{}
	s.db = dynamo.New(session.New(), &aws.Config{})
	s.table = s.db.Table(util.GetEnv("DYNAMODB_TABLE", "Services"))
	return &s
}

func (s *Service) InitDB(apiVersion string) error {
	ds := DynamoServices{ApiVersion: apiVersion, ServiceName: "__SERVICES", Time: "0", Version: 1, Services: []*DynamoServicesElement{}}

	// __SERVICE not found, write first record
	err := s.table.Put(ds).Run()

	if err != nil {
		serviceLogger.Errorf("Error during put of first record: %v", err.Error())
		return err
	}
	return nil
}

func (s *Service) initService(dsElement *DynamoServicesElement) error {
	ds := DynamoServices{ServiceName: "__SERVICES", Time: "0", Version: 1, Services: []*DynamoServicesElement{dsElement}}

	// __SERVICE not found, write first record
	err := s.table.Put(ds).Run()

	if err != nil {
		serviceLogger.Errorf("Error during put of first record: %v", err.Error())
		return err
	}
	return nil
}

func (s *Service) GetServices(ds *DynamoServices) error {
	err := s.table.Get("ServiceName", "__SERVICES").Range("Time", dynamo.Equal, "0").One(ds)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case dynamodb.ErrCodeProvisionedThroughputExceededException:
				serviceLogger.Errorf(dynamodb.ErrCodeProvisionedThroughputExceededException, aerr.Error())
			case dynamodb.ErrCodeResourceNotFoundException:
				serviceLogger.Errorf(dynamodb.ErrCodeResourceNotFoundException, aerr.Error())
			case dynamodb.ErrCodeInternalServerError:
				serviceLogger.Errorf(dynamodb.ErrCodeInternalServerError, aerr.Error())
			case "ValidationException":
				serviceLogger.Errorf("%v", aerr.Error())
			default:
				serviceLogger.Errorf(aerr.Error())
			}
		} else {
			return err
		}
		serviceLogger.Errorf("Error during get: %v", err.Error())
		return err
	}
	return nil
}
func (s *Service) CreateService(dsElement *DynamoServicesElement) error {
	// check input
	if (s.ServiceName == "") || (s.ClusterName == "") {
		serviceLogger.Errorf("Couldn't add %v (cluster = %v, listener # = %d)", s.ServiceName, s.ClusterName, len(s.Listeners))
		return errors.New("Couldn't add " + s.ServiceName + ": cluster / listeners is empty")
	}

	var ds DynamoServices
	err := s.GetServices(&ds)
	if err != nil {
		if err.Error() == "dynamo: no item found" {
			// service needs to be initialized
			serviceLogger.Debugf("Item not found: writing first __SERVICE record")
			err = s.initService(dsElement)
			if err != nil {
				return err
			}
			// record is written, return
			return nil
		} else {
			serviceLogger.Errorf(err.Error())
			return err
		}
	}

	retry := true
	for y := 0; y < 4 && retry; y++ {
		// add new service
		o := false
		for i, a := range ds.Services {
			if a.S == dsElement.S {
				ds.Services[i] = dsElement
				o = true
			}
		}
		if !o {
			ds.Services = append(ds.Services, dsElement)
		}
		ds.Version += 1

		// do a conditional put, where version
		serviceLogger.Debugf("Putting new services record with version %v", ds.Version)
		err = s.table.Put(ds).If("$ = ?", "Version", ds.Version-1).Run()

		if err != nil {
			if aerr, ok := err.(awserr.Error); ok {
				switch aerr.Code() {
				case dynamodb.ErrCodeConditionalCheckFailedException:
					serviceLogger.Debugf("Conditional check failed - retrying (%v)", aerr.Error())
					err = s.GetServices(&ds)
					if err != nil {
						return err
					}
				default:
					serviceLogger.Errorf("Error during put of first record: %v", aerr.Error())
					return err
				}
			} else {
				serviceLogger.Errorf("Error during put of first record: %v", err.Error())
				return err
			}
		} else {
			retry = false
			return nil
		}
	}
	return nil
}
func (s *Service) ServiceExistsInDynamo() (bool, error) {
	var ds DynamoServices
	err := s.GetServices(&ds)
	if err != nil {
		serviceLogger.Errorf(err.Error())
		return false, err
	}
	for _, a := range ds.Services {
		if a.S == s.ServiceName {
			return true, nil
		}
	}
	return false, nil
}
func (s *Service) NewDeployment(taskDefinitionArn *string, d *Deploy) (*DynamoDeployment, error) {
	day := time.Now().Format("2006-01-02")
	month := time.Now().Format("2006-01")
	w := DynamoDeployment{ServiceName: s.ServiceName, Time: time.Now(), Day: day, Month: month, TaskDefinitionArn: taskDefinitionArn, DeployData: d, Status: "running", Version: 1}

	lastDeploy, err := s.GetLastDeploy()
	if err != nil {
		w.Scaling.DesiredCount = d.DesiredCount
	} else {
		w.Scaling = lastDeploy.Scaling
		w.Scaling.DesiredCount = util.Max(d.DesiredCount, lastDeploy.Scaling.DesiredCount)
	}

	err = s.table.Put(w).Run()

	if err != nil {
		serviceLogger.Errorf("Error during put: %v", err.Error())
		return nil, err
	}
	return &w, nil
}
func (s *Service) GetLastDeploy() (*DynamoDeployment, error) {
	var dd DynamoDeployment
	if s.ServiceName == "" {
		return nil, errors.New("serviceName not set")
	}
	err := s.table.Get("ServiceName", s.ServiceName).Range("Time", dynamo.LessOrEqual, time.Now()).Order(dynamo.Descending).Limit(1).One(&dd)
	if err != nil {
		if err.Error() == "dynamo: no item found" {
			return nil, errors.New("NoItemsFound: no items found")
		}
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case dynamodb.ErrCodeProvisionedThroughputExceededException:
				serviceLogger.Errorf(dynamodb.ErrCodeProvisionedThroughputExceededException, aerr.Error())
			case dynamodb.ErrCodeResourceNotFoundException:
				serviceLogger.Errorf(dynamodb.ErrCodeResourceNotFoundException, aerr.Error())
			case dynamodb.ErrCodeInternalServerError:
				serviceLogger.Errorf(dynamodb.ErrCodeInternalServerError, aerr.Error())
			case "ValidationException":
				serviceLogger.Errorf("%v", aerr.Error())
			default:
				serviceLogger.Errorf(aerr.Error())
			}
		} else {
			return nil, err
		}
		serviceLogger.Errorf("Error during get: %v", err.Error())
		return nil, err
	}
	serviceLogger.Debugf("Retrieved last deployment %v at %v", dd.ServiceName, dd.Time)
	return &dd, nil
}
func (s *Service) GetDeploys(action string, limit int64) ([]DynamoDeployment, error) {
	var dds []DynamoDeployment
	// add date to table
	switch {
	case action == "byMonth":
		for i := 0; i < 3; i++ {
			var dd []DynamoDeployment
			serviceLogger.Debugf("Retrieving records from: %v", time.Now().AddDate(0, i*-1, 0).Format("2006-01"))
			err := s.table.Get("Month", time.Now().AddDate(0, i*-1, 0).Format("2006-01")).Index("MonthIndex").Range("Time", dynamo.LessOrEqual, time.Now()).Order(dynamo.Descending).Limit(limit).All(&dd)
			dds = append(dds, dd...)
			if err != nil {
				return dds, err
			}
			if int64(len(dds)) >= limit {
				return dds[0:limit], nil
			}
		}
	case action == "byDay":
		for i := 0; i < 3; i++ {
			var dd []DynamoDeployment
			serviceLogger.Debugf("Retrieving records from: %v", time.Now().AddDate(0, 0, i*-1).Format("2006-01-02"))
			err := s.table.Get("Day", time.Now().AddDate(0, 0, i*-1).Format("2006-01-02")).Index("DayIndex").Range("Time", dynamo.LessOrEqual, time.Now()).Order(dynamo.Descending).Limit(limit).All(&dd)
			dds = append(dds, dd...)
			if err != nil {
				return dds, err
			}
			if int64(len(dds)) >= limit {
				return dds[0:limit], nil
			}
		}
	case action == "secondToLast":
		var dd []DynamoDeployment
		serviceLogger.Debugf("Retrieving second last deploy")
		err := s.table.Get("ServiceName", s.ServiceName).Range("Time", dynamo.LessOrEqual, time.Now()).Order(dynamo.Descending).Limit(2).All(&dd)
		if err != nil {
			return dds, err
		}
		if len(dd) != 2 {
			return nil, errors.New("NoSecondToLast: No second to last deployment")
		}
		dds = dd[1:2]
	default:
		return nil, errors.New("No action specified")
	}
	return dds, nil
}
func (s *Service) GetDeploysForService(serviceName string) ([]DynamoDeployment, error) {
	var dds []DynamoDeployment
	serviceLogger.Debugf("Retrieving records for: %v", serviceName)
	err := s.table.Get("ServiceName", serviceName).Range("Time", dynamo.LessOrEqual, time.Now()).Order(dynamo.Descending).Limit(20).All(&dds)
	return dds, err
}

func (s *Service) SetDeploymentStatus(dd *DynamoDeployment, status string) error {
	var err error
	dd.Version = dd.Version + 1
	dd.Status = status

	serviceLogger.Debugf("Setting status of service %v_%v to %v", dd.ServiceName, dd.Time.Format("2006-01-02T15:04:05-0700"), status)

	if dd.Version > 1 {
		err = s.table.Put(dd).If("$ = ?", "Version", (dd.Version - 1)).Run()
	} else {
		// version was not set, don't use version conditional
		err = s.table.Put(dd).Run()
	}

	if err != nil {
		serviceLogger.Errorf("Error during put: %v", err.Error())
		return err
	}
	return nil
}
func (s *Service) SetDeploymentStatusWithReason(dd *DynamoDeployment, status, reason string) error {
	var err error
	dd.Version = dd.Version + 1
	dd.Status = status
	dd.DeployError = reason

	serviceLogger.Debugf("Setting status of service %v_%v to %v", dd.ServiceName, dd.Time.Format("2006-01-02T15:04:05-0700"), status)

	if dd.Version > 1 {
		err = s.table.Put(dd).If("$ = ?", "Version", (dd.Version - 1)).Run()
	} else {
		// version was not set, don't use version conditional
		err = s.table.Put(dd).Run()
	}

	if err != nil {
		serviceLogger.Errorf("Error during put: %v", err.Error())
		return err
	}
	return nil
}
func (s *Service) GetDeployment(serviceName string, strTime string) (*DynamoDeployment, error) {
	var dd DynamoDeployment

	layout := "2006-01-02T15:04:05.9Z"
	t, err := time.Parse(layout, strTime)

	if err != nil {
		serviceLogger.Errorf("Could not parse %v from string to time", strTime)
		return nil, err
	}

	serviceLogger.Debugf("Retrieving deployment of service %v_%v", serviceName, strTime)
	err = s.table.Get("ServiceName", serviceName).Range("Time", dynamo.Equal, t).Limit(1).One(&dd)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			serviceLogger.Errorf(aerr.Error())
		} else {
			return nil, err
		}
		serviceLogger.Errorf("Error during get: %v", err.Error())
		return nil, err
	}
	serviceLogger.Debugf("Retrieved deployment %v_%v with status %v", dd.ServiceName, dd.Time, dd.Status)

	return &dd, nil
}

func (s *Service) GetServiceVersionsByTags(serviceName, imageName string, tags map[string]string) ([]ServiceVersion, error) {
	var svs []ServiceVersion
	var dds []DynamoDeployment

	matched := make(map[string]bool)

	serviceLogger.Debugf("Retrieving records for: %v, imageId: %v", serviceName, imageName)
	err := s.table.Get("ServiceName", serviceName).Range("Time", dynamo.LessOrEqual, time.Now()).Order(dynamo.Descending).Limit(int64(math.Max(float64(100), float64(len(tags))))).All(&dds)
	for _, dd := range dds {
		for _, container := range dd.DeployData.Containers {
			// determine containerTag
			containerTag := ""
			if container.ContainerURI != "" {
				split := strings.Split(container.ContainerURI, ":")
				if len(split) == 2 {
					containerTag = split[1]
				} else {
					containerTag = "latest"
				}
			} else {
				containerTag = container.ContainerTag
			}
			// Populate lastdeploy with matching images
			if container.ContainerImage == imageName || (container.ContainerImage == "" && dd.ServiceName == imageName) {
				for tag, imageId := range tags {
					if tag == containerTag {
						if _, ok := matched[tag]; !ok {
							svs = append(svs, ServiceVersion{ImageName: imageName, Tag: tag, ImageId: imageId, LastDeploy: dd.Time})
							matched[tag] = true
						}
					}
				}
			}
		}
	}
	return svs, err

}

func (s *Service) CreateTable() error {
	err := s.db.CreateTable(util.GetEnv("DYNAMODB_TABLE", "Services"), DynamoDeployment{}).
		Provision(2, 1).
		ProvisionIndex("DayIndex", 1, 1).
		ProvisionIndex("MonthIndex", 1, 1).
		Run()
	if err != nil {
		return err
	}

	s.table = s.db.Table(util.GetEnv("DYNAMODB_TABLE", "Services"))
	return nil
}
func (s *Service) GetClusterName() (string, error) {
	var clusterName string
	var ds DynamoServices
	serviceLogger.Debugf("Going to determine clusterName of %v", s.ServiceName)
	err := s.GetServices(&ds)
	if err != nil {
		return clusterName, err
	}
	for _, v := range ds.Services {
		if v.S == s.ServiceName {
			clusterName = v.C
		}
	}
	if clusterName == "" {
		return clusterName, errors.New("Service not found")
	}
	return clusterName, nil
}
func (s *Service) SetScalingProperty(desiredCount int64) error {
	dd, err := s.GetLastDeploy()
	dd.Version = dd.Version + 1
	dd.Scaling.DesiredCount = desiredCount

	if dd.Version > 1 {
		err = s.table.Put(dd).If("$ = ?", "Version", (dd.Version - 1)).Run()
	} else {
		// version was not set, don't use version conditional
		err = s.table.Put(dd).If("$ = ?", "Status", dd.Status).Run()
	}

	if err != nil {
		serviceLogger.Errorf("Error during put: %v", err.Error())
		return err
	}
	return nil
}
func (s *Service) SetAutoscalingProperties(desiredCount int64, resourceId string, policyNames []string) error {
	dd, err := s.GetLastDeploy()
	dd.Version = dd.Version + 1
	dd.Scaling.DesiredCount = desiredCount
	dd.Scaling.Autoscaling.ResourceId = resourceId
	dd.Scaling.Autoscaling.PolicyNames = policyNames

	if dd.Version > 1 {
		err = s.table.Put(dd).If("$ = ?", "Version", (dd.Version - 1)).Run()
	} else {
		// version was not set, don't use version conditional
		err = s.table.Put(dd).If("$ = ?", "Status", dd.Status).Run()
	}

	if err != nil {
		serviceLogger.Errorf("Error during put: %v", err.Error())
		return err
	}
	return nil
}
func (s *Service) SetManualTasksArn(manualTaskArn string) error {
	dd, err := s.GetLastDeploy()
	dd.ManualTasksArns = append(dd.ManualTasksArns, manualTaskArn)
	dd.Version = dd.Version + 1

	if dd.Version > 1 {
		err = s.table.Put(dd).If("$ = ?", "Version", (dd.Version - 1)).Run()
	} else {
		// version was not set, don't use version conditional
		err = s.table.Put(dd).If("$ = ?", "Status", dd.Status).Run()
	}

	if err != nil {
		serviceLogger.Errorf("Error during put: %v", err.Error())
		return err
	}
	return nil
}
func (s *Service) UpdateServiceLimits(clusterName, serviceName string, cpuReservation, cpuLimit, memoryReservation, memoryLimit int64) error {
	var dss DynamoServices
	var found bool
	err := s.GetServices(&dss)
	if err != nil {
		return err
	}
	for i, ds := range dss.Services {
		if ds.C == clusterName && ds.S == serviceName {
			found = true
			dss.Services[i].CpuReservation = cpuReservation
			dss.Services[i].CpuLimit = cpuLimit
			dss.Services[i].MemoryReservation = memoryReservation
			dss.Services[i].MemoryLimit = memoryLimit
		}
	}
	if !found {
		return errors.New("Couldn't update service limits: Service not found")
	}
	dss.Version = dss.Version + 1
	return s.table.Put(dss).If("$ = ?", "Version", dss.Version-1).Run()
}
func (s *Service) UpdateServiceListeners(clusterName, serviceName string, listeners []string) error {
	var dss DynamoServices
	var found bool
	err := s.GetServices(&dss)
	if err != nil {
		return err
	}
	for i, ds := range dss.Services {
		if ds.C == clusterName && ds.S == serviceName {
			found = true
			dss.Services[i].Listeners = listeners
		}
	}
	if !found {
		return errors.New("Couldn't update service listener: Service not found")
	}
	dss.Version = dss.Version + 1
	return s.table.Put(dss).If("$ = ?", "Version", dss.Version-1).Run()
}
func (s *Service) GetApiVersion() (string, error) {
	var dss DynamoServices
	err := s.GetServices(&dss)
	if err != nil {
		return "", err
	}
	return dss.ApiVersion, nil
}
func (s *Service) SetApiVersion(apiVersion string) error {
	var dss DynamoServices
	err := s.GetServices(&dss)
	if err != nil {
		return err
	}
	dss.Version = dss.Version + 1
	dss.ApiVersion = apiVersion
	return s.table.Put(dss).If("$ = ?", "Version", dss.Version-1).Run()
}

func (s *Service) GetClusterInfo() (*DynamoCluster, error) {
	var dc DynamoCluster
	err := s.table.Get("ServiceName", "__CLUSTERS").Range("Time", dynamo.LessOrEqual, time.Now()).Order(dynamo.Descending).Limit(1).One(&dc)
	if err != nil {
		if err.Error() == "dynamo: no item found" {
			return nil, nil
		}
		if aerr, ok := err.(awserr.Error); ok {
			serviceLogger.Errorf(aerr.Error())
		} else {
			serviceLogger.Errorf(err.Error())
		}
		return nil, err
	}
	return &dc, nil
}
func (s *Service) PutClusterInfo(dc DynamoCluster, clusterName string, action string, pendingAction string) (*DynamoCluster, error) {
	dc.ScalingOperation = DynamoClusterScalingOperation{ClusterName: clusterName, Action: action, PendingAction: pendingAction}
	dc.Identifier = "__CLUSTERS"
	dc.Time = time.Now()
	dc.ExpirationTime = time.Now().AddDate(0, 0, 30)
	dc.ExpirationTimeTTL = dc.ExpirationTime.Unix()
	err := s.table.Put(dc).Run()
	if err != nil {
		if err.Error() == "dynamo: no item found" {
			return nil, nil
		}
		if aerr, ok := err.(awserr.Error); ok {
			serviceLogger.Errorf(aerr.Error())
		} else {
			serviceLogger.Errorf(err.Error())
		}
		return nil, err
	}
	return &dc, nil
}
func (s *Service) GetScalingActivity(clusterName string, startTime time.Time) (string, string, error) {
	var dcs []DynamoCluster
	err := s.table.Get("ServiceName", "__CLUSTERS").Range("Time", dynamo.GreaterOrEqual, startTime).All(&dcs)
	if err != nil {
		if err.Error() == "dynamo: no item found" {
			return "", "", nil
		}
		if aerr, ok := err.(awserr.Error); ok {
			serviceLogger.Errorf(aerr.Error())
		} else {
			serviceLogger.Errorf(err.Error())
		}
		return "", "", err
	}
	for _, dc := range dcs {
		// check actions
		if dc.ScalingOperation.ClusterName == clusterName && dc.ScalingOperation.Action != "no" {
			serviceLogger.Debugf("Found a previous scaling operation (action %v, start time: %v)", dc.ScalingOperation.Action, startTime.UTC().Format("2006-01-02T15:04:05-0700"))
			return dc.ScalingOperation.Action, "", nil
		}
		// check pending actions
		if dc.ScalingOperation.ClusterName == clusterName && dc.ScalingOperation.PendingAction != "" {
			serviceLogger.Debugf("Found a previous pending scaling operation (action %v, start time: %v)", dc.ScalingOperation.PendingAction, startTime.UTC().Format("2006-01-02T15:04:05-0700"))
			return "no", dc.ScalingOperation.PendingAction, nil
		}
	}
	return "no", "", nil
}

func (s *Service) IsDeployRunning() (bool, error) {
	var deployRunning bool
	lastDeploys, err := s.GetDeploys("byDay", 50)
	if err != nil {
		return false, err
	}
	for _, v := range lastDeploys {
		if v.Status == "running" {
			deployRunning = true
		}
	}
	return deployRunning, nil
}

func (s *Service) AutoscalingPullInit() error {
	p := &DynamoAutoscalingPull{Identifier: "__AUTOSCALINGPULL", Time: "0", Lock: "initial"}
	err := s.table.Put(p).If("attribute_not_exists(L)").Run()

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case dynamodb.ErrCodeConditionalCheckFailedException:
				return nil
			default:
				serviceLogger.Errorf("Error during put of first record: %v", aerr.Error())
				return err
			}
		} else {
			serviceLogger.Errorf("Error during put of first record: %v", err.Error())
			return err
		}
	}
	serviceLogger.Infof("initialized autoscalingPull in backend")
	return nil
}
func (s *Service) AutoscalingPullAcquireLock(localId string) (bool, error) {
	p := &DynamoAutoscalingPull{Identifier: "__AUTOSCALINGPULL", Time: "0", Lock: localId, LockTimestamp: time.Now()}
	err := s.table.Put(p).If("$ < ?", "LT", time.Now().Add(-1*time.Minute)).Run()
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case dynamodb.ErrCodeConditionalCheckFailedException:
				return false, nil
			default:
				return false, err
			}
		} else {
			return false, err
		}
	}
	return true, nil
}
