package ecs

import (
	"testing"

	"github.com/in4it/ecs-deploy/util"
)

func TestListImages(t *testing.T) {
	if accountId == nil {
		t.Skip(noAWSMsg)
	}
	ecr := ECR{}
	imageName := util.GetEnv("TEST_IMAGENAME", "ecs-deploy")
	_, err := ecr.listImagesWithTag(imageName)
	if err != nil {
		t.Errorf("Error: %v", err)
		return
	}
}
