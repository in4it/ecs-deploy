package ecs

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/juju/loggo"
)

// logging
var ecrLogger = loggo.GetLogger("ecr")

// ECR struct
type ECR struct {
	RepositoryName, RepositoryURI string
}

// Creates ECR repository
func (e *ECR) CreateRepository() error {
	svc := ecr.New(session.New())
	input := &ecr.CreateRepositoryInput{
		RepositoryName: aws.String(e.RepositoryName),
	}

	res, err := svc.CreateRepository(input)
	if err == nil && res.Repository.RepositoryUri != nil {
		e.RepositoryURI = *res.Repository.RepositoryUri
		return nil
	} else {
		return err
	}
}
func (e *ECR) ListImagesWithTag(repositoryName string) (map[string]string, error) {
	svc := ecr.New(session.New())

	images := make(map[string]string)

	input := &ecr.ListImagesInput{
		RepositoryName: aws.String(repositoryName),
	}

	pageNum := 0
	err := svc.ListImagesPages(input,
		func(page *ecr.ListImagesOutput, lastPage bool) bool {
			pageNum++
			for _, image := range page.ImageIds {
				if image.ImageTag != nil {
					images[*image.ImageTag] = *image.ImageDigest
				}
			}
			return pageNum <= 100
		})

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			ecrLogger.Errorf(aerr.Error())
		} else {
			ecrLogger.Errorf(err.Error())
		}
		return images, err
	}
	return images, nil
}
