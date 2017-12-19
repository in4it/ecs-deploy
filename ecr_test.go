package main

import (
	"fmt"
	"testing"
)

func TestListImages(t *testing.T) {
	if accountId == nil {
		t.Skip(noAWSMsg)
	}
	ecr := ECR{}
	imageName := getEnv("TEST_IMAGENAME", "ecs-deploy")
	result, err := ecr.listImagesWithTag(imageName)
	if err != nil {
		t.Errorf("Error: %v", err)
		return
	}
}
