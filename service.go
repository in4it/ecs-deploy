package main

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/guregu/dynamo"
	"github.com/juju/loggo"

	"errors"
	"math"
	"time"
)

// logging
var serviceLogger = loggo.GetLogger("service")

type Service struct {
	db          *dynamo.DB
	table       dynamo.Table
	serviceName string
	clusterName string
	listeners   []string
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
}

// dynamo services struct
type DynamoServices struct {
	ServiceName string `dynamo:"ServiceName,hash"`
	Services    []*DynamoServicesElement
	Time        string `dynamo:"Time,range"`
	Version     int64
}
type DynamoServicesElement struct {
	C string
	S string
	L []string
}

func newService() *Service {
	s := Service{}
	s.db = dynamo.New(session.New(), &aws.Config{})
	s.table = s.db.Table(getEnv("DYNAMODB_TABLE", "Services"))
	return &s
}

func (s *Service) initService(dsElement *DynamoServicesElement) error {
	ds := &DynamoServices{ServiceName: "__SERVICES", Time: "0", Version: 1, Services: []*DynamoServicesElement{dsElement}}

	// __SERVICE not found, write first record
	err := s.table.Put(ds).Run()

	if err != nil {
		serviceLogger.Errorf("Error during put of first record: %v", err.Error())
		return err
	}
	return nil
}

func (s *Service) getServices(ds *DynamoServices) error {
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
func (s *Service) createService() error {
	// check input
	if (s.serviceName == "") || (s.clusterName == "") || (len(s.listeners) == 0) {
		serviceLogger.Errorf("Couldn't add %v (cluster = %v, listener # = %d)", s.serviceName, s.clusterName, len(s.listeners))
		return errors.New("Couldn't add " + s.serviceName + ": cluster / listeners is empty")
	}

	var ds DynamoServices
	dsElement := &DynamoServicesElement{S: s.serviceName, C: s.clusterName, L: s.listeners}

	err := s.getServices(&ds)
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
					err = s.getServices(&ds)
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
func (s *Service) newDeployment(taskDefinitionArn *string, d *Deploy) (*DynamoDeployment, error) {
	day := time.Now().Format("2006-01-02")
	month := time.Now().Format("2006-01")
	w := DynamoDeployment{ServiceName: s.serviceName, Time: time.Now(), Day: day, Month: month, TaskDefinitionArn: taskDefinitionArn, DeployData: d, Status: "running", Version: 1}

	lastDeploy, err := s.getLastDeploy()
	if err != nil {
		w.Scaling.DesiredCount = d.DesiredCount
	} else {
		w.Scaling = lastDeploy.Scaling
		w.Scaling.DesiredCount = Max(d.DesiredCount, lastDeploy.Scaling.DesiredCount)
	}

	err = s.table.Put(w).Run()

	if err != nil {
		serviceLogger.Errorf("Error during put: %v", err.Error())
		return nil, err
	}
	return &w, nil
}
func (s *Service) getLastDeploy() (*DynamoDeployment, error) {
	var dd DynamoDeployment
	if s.serviceName == "" {
		return nil, errors.New("serviceName not set")
	}
	err := s.table.Get("ServiceName", s.serviceName).Range("Time", dynamo.LessOrEqual, time.Now()).Order(dynamo.Descending).Limit(1).One(&dd)
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
func (s *Service) getDeploys(action string, limit int) ([]DynamoDeployment, error) {
	var dds []DynamoDeployment
	// add date to table
	var dd []DynamoDeployment
	switch {
	case action == "byMonth":
		for i := 0; i < 3; i++ {
			serviceLogger.Debugf("Retrieving records from: %v", time.Now().AddDate(0, i*-1, 0).Format("2006-01"))
			err := s.table.Get("Month", time.Now().AddDate(0, i*-1, 0).Format("2006-01")).Index("MonthIndex").Range("Time", dynamo.LessOrEqual, time.Now()).Order(dynamo.Descending).Limit(20).All(&dd)
			dds = append(dds, dd...)
			if err != nil {
				return dds, err
			}
			if len(dds) >= limit {
				return dds[0:limit], nil
			}
		}
	case action == "secondToLast":
		serviceLogger.Debugf("Retrieving second last deploy")
		err := s.table.Get("ServiceName", s.serviceName).Range("Time", dynamo.LessOrEqual, time.Now()).Order(dynamo.Descending).Limit(2).All(&dd)
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
func (s *Service) getDeploysForService(serviceName string) ([]DynamoDeployment, error) {
	var dds []DynamoDeployment
	serviceLogger.Debugf("Retrieving records for: %v", serviceName)
	err := s.table.Get("ServiceName", serviceName).Range("Time", dynamo.LessOrEqual, time.Now()).Order(dynamo.Descending).Limit(20).All(&dds)
	return dds, err
}

func (s *Service) setDeploymentStatus(d *DynamoDeployment, status string) error {

	serviceLogger.Debugf("Setting status of service %v_%v to %v", d.ServiceName, d.Time.Format("2006-01-02T15:04:05-0700"), status)
	d.Status = status
	err := s.table.Put(d).Run()

	if err != nil {
		serviceLogger.Errorf("Error during put: %v", err.Error())
		return err
	}
	return nil
}
func (s *Service) setDeploymentStatusWithReason(d *DynamoDeployment, status, reason string) error {

	serviceLogger.Debugf("Setting status of service %v_%v to %v", d.ServiceName, d.Time.Format("2006-01-02T15:04:05-0700"), status)
	d.Status = status
	d.DeployError = reason
	err := s.table.Put(d).Run()

	if err != nil {
		serviceLogger.Errorf("Error during put: %v", err.Error())
		return err
	}
	return nil
}
func (s *Service) getDeployment(serviceName string, strTime string) (*DynamoDeployment, error) {
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

func (s *Service) getServiceVersionsByTags(serviceName, imageName string, tags map[string]string) ([]ServiceVersion, error) {
	var svs []ServiceVersion
	var dds []DynamoDeployment

	matched := make(map[string]bool)

	serviceLogger.Debugf("Retrieving records for: %v, imageId: %v", serviceName, imageName)
	err := s.table.Get("ServiceName", serviceName).Range("Time", dynamo.LessOrEqual, time.Now()).Order(dynamo.Descending).Limit(int64(math.Max(float64(100), float64(len(tags))))).All(&dds)
	for _, dd := range dds {
		for _, container := range dd.DeployData.Containers {
			if container.ContainerImage == imageName || (container.ContainerImage == "" && dd.ServiceName == imageName) {
				for tag, imageId := range tags {
					if tag == container.ContainerTag {
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

func (s *Service) createTable() error {
	err := s.db.CreateTable(getEnv("DYNAMODB_TABLE", "Services"), DynamoDeployment{}).
		Provision(2, 1).
		ProvisionIndex("DayIndex", 1, 1).
		ProvisionIndex("MonthIndex", 1, 1).
		Run()
	if err != nil {
		return err
	}

	s.table = s.db.Table(getEnv("DYNAMODB_TABLE", "Services"))
	return nil
}
func (s *Service) getClusterName() (string, error) {
	var clusterName string
	var ds DynamoServices
	serviceLogger.Debugf("Going to determine clusterName of %v", s.serviceName)
	err := s.getServices(&ds)
	if err != nil {
		return clusterName, err
	}
	for _, v := range ds.Services {
		if v.S == s.serviceName {
			clusterName = v.C
		}
	}
	if clusterName == "" {
		return clusterName, errors.New("Service not found")
	}
	return clusterName, nil
}
func (s *Service) setScalingProperty(desiredCount int64) error {
	dd, err := s.getLastDeploy()
	dd.Scaling.DesiredCount = desiredCount

	err = s.table.Put(dd).If("$ = ?", "Status", dd.Status).Run()

	if err != nil {
		serviceLogger.Errorf("Error during put: %v", err.Error())
		return err
	}
	return nil
}
func (s *Service) setManualTasksArn(manualTaskArn string) error {
	dd, err := s.getLastDeploy()
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
