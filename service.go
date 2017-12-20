package main

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/guregu/dynamo"
	"github.com/juju/loggo"

	"errors"
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
	ServiceName       string
	Time              time.Time
	Day               string
	Month             string
	Status            string
	Tag               string
	TaskDefinitionArn *string
	DeployData        *Deploy
}

// dynamo services struct
type DynamoServices struct {
	ServiceName string
	Services    []*DynamoServicesElement
	Time        string
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
	day := time.Now().Format("2006-01-01")
	month := time.Now().Format("2006-01")
	w := DynamoDeployment{ServiceName: s.serviceName, Time: time.Now(), Day: day, Month: month, TaskDefinitionArn: taskDefinitionArn, DeployData: d, Status: "running"}
	err := s.table.Put(w).Run()

	if err != nil {
		serviceLogger.Errorf("Error during put: %v", err.Error())
		return nil, err
	}
	return &w, nil
}
func (s *Service) getLastDeploy() (*DynamoDeployment, error) {
	var dd DynamoDeployment
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
func (s *Service) getDeploymentStatus(serviceName string, strTime string) (*DynamoDeployment, error) {
	var dd DynamoDeployment

	layout := "2006-01-02T15:04:05.9Z"
	t, err := time.Parse(layout, strTime)

	if err != nil {
		serviceLogger.Errorf("Could not parse %v from string to time", strTime)
		return nil, err
	}

	serviceLogger.Debugf("Retrieving status of service %v_%v", serviceName, strTime)
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
