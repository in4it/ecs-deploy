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
	serviceName string
	clusterName string
	listeners   []string
}

type DynamoDeployment struct {
	ServiceName       string
	Time              time.Time
	Day               string
	Month             string
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

func (s *Service) initService(dsElement *DynamoServicesElement) error {
	db := dynamo.New(session.New(), &aws.Config{})
	table := db.Table("Services")

	ds := &DynamoServices{ServiceName: "__SERVICES", Time: "0", Version: 1, Services: []*DynamoServicesElement{dsElement}}

	// __SERVICE not found, write first record
	err := table.Put(ds).Run()

	if err != nil {
		serviceLogger.Errorf("Error during put of first record: %v", err.Error())
		return err
	}
	return nil
}

func (s *Service) getServices(ds *DynamoServices) error {
	db := dynamo.New(session.New(), &aws.Config{})
	table := db.Table("Services")
	err := table.Get("ServiceName", "__SERVICES").Range("Time", dynamo.Equal, "0").One(ds)
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
	db := dynamo.New(session.New(), &aws.Config{})
	table := db.Table("Services")

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
		err = table.Put(ds).If("$ = ?", "Version", ds.Version-1).Run()

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
func (s *Service) newDeployment(taskDefinitionArn *string, d *Deploy) error {
	db := dynamo.New(session.New(), &aws.Config{})
	table := db.Table("Services")
	day := time.Now().Format("2006-01-01")
	month := time.Now().Format("2006-01")
	w := DynamoDeployment{ServiceName: s.serviceName, Time: time.Now(), Day: day, Month: month, TaskDefinitionArn: taskDefinitionArn, DeployData: d}
	err := table.Put(w).Run()

	if err != nil {
		serviceLogger.Errorf("Error during put: %v", err.Error())
		return err
	}
	return nil
}
func (s *Service) getLastDeploy() (*DynamoDeployment, error) {
	var dd DynamoDeployment
	db := dynamo.New(session.New(), &aws.Config{})
	table := db.Table("Services")
	err := table.Get("ServiceName", s.serviceName).Range("Time", dynamo.LessOrEqual, time.Now()).Order(dynamo.Descending).Limit(1).One(&dd)
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
			return nil, err
		}
		serviceLogger.Errorf("Error during get: %v", err.Error())
		return nil, err
	}
	serviceLogger.Debugf("Retrieved last deployment %v at %v", dd.ServiceName, dd.Time)
	return &dd, nil
}
func (s *Service) getDeploys() ([]DynamoDeployment, error) {
	var dds []DynamoDeployment
	db := dynamo.New(session.New(), &aws.Config{})
	table := db.Table("Services")
	// add date to table
	for i := 0; i < 3; i++ {
		var dd []DynamoDeployment
		serviceLogger.Debugf("Retrieving records from: %v", time.Now().AddDate(0, i*-1, 0).Format("2006-01"))
		err := table.Get("Month", time.Now().AddDate(0, i*-1, 0).Format("2006-01")).Index("MonthIndex").Range("Time", dynamo.LessOrEqual, time.Now()).Order(dynamo.Descending).Limit(20).All(&dd)
		dds = append(dds, dd...)
		if err != nil {
			return dds, err
		}
		if len(dds) == 20 {
			return dds, nil
		}
	}
	return dds, nil
}
func (s *Service) getDeploysForService(serviceName string) ([]DynamoDeployment, error) {
	var dds []DynamoDeployment
	db := dynamo.New(session.New(), &aws.Config{})
	table := db.Table("Services")
	serviceLogger.Debugf("Retrieving records for: %v", serviceName)
	err := table.Get("ServiceName", serviceName).Range("Time", dynamo.LessOrEqual, time.Now()).Order(dynamo.Descending).Limit(20).All(&dds)
	return dds, err
}
