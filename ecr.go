package main

import (
  "github.com/aws/aws-sdk-go/aws"
  "github.com/aws/aws-sdk-go/aws/session"
  "github.com/aws/aws-sdk-go/service/ecr"
)

// ECR struct
type ECR struct {
  repositoryName, repositoryURI string
}

// Creates ECR repository
func (e *ECR) createRepository() (error) {
  svc := ecr.New(session.New())
  input := &ecr.CreateRepositoryInput{
      RepositoryName: aws.String(e.repositoryName),
  }

  res, err := svc.CreateRepository(input)
  if err == nil && res.Repository.RepositoryUri != nil {
    e.repositoryURI = *res.Repository.RepositoryUri
    return nil
  } else {
    return err
  }
}

