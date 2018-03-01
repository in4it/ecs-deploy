package ecs

import (
	"fmt"
	"testing"

	"github.com/in4it/ecs-deploy/util"
)

func TestListImages(t *testing.T) {
	if accountId == nil {
		t.Skip(noAWSMsg)
	}
	ecr := ECR{}
	imageName := util.GetEnv("TEST_IMAGENAME", "ecs-deploy")
	_, err := ecr.ListImagesWithTag(imageName)
	if err != nil {
		t.Errorf("Error: %v", err)
		return
	}
}
func TestRepositoryExists(t *testing.T) {
	if accountId == nil {
		t.Skip(noAWSMsg)
	}
	ecr := ECR{}
	imageName := util.GetEnv("TEST_IMAGENAME", "ecs-deploy")
	res, err := ecr.RepositoryExists(imageName)
	fmt.Printf("Repository %v exists: %v\n", imageName, res)
	if err != nil {
		t.Errorf("Error: %v", err)
		return
	}
}
