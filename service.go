package main

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/guregu/dynamo"
)

type Service struct {
	ServiceName      string
	ECRRepositoryURI string
}
type DynamoService struct {
	ServiceName      string
	ECRRepositoryURI string `dynamo:"ECR"`
}

func (s *Service) createService() {
	db := dynamo.New(session.New(), &aws.Config{})
	table := db.Table("Services")

	var result Service
	err := table.Get("ServiceName", s.ServiceName).One(&result)
	if err != nil {
		//fmt.Println(err.Error())
		return
	}

	if result == (Service{}) {
		// service not found, write new service
		w := DynamoService{ServiceName: s.ServiceName, ECRRepositoryURI: s.ECRRepositoryURI}
		err := table.Put(w).Run()

		if err != nil {
			//fmt.Println(err.Error())
			return
		}
	} else {
		// service found
		s.ECRRepositoryURI = result.ECRRepositoryURI
	}
}
