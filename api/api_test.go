package api

import (
	"testing"
)

func TestDeployServiceValidator(t *testing.T) {
	// api object
	a := API{}

	// test with 2 characters
	d := Deploy{
		Containers: []*DeployContainer{
			{
				ContainerName: "abc",
			},
		},
	}
	serviceName := "ab"
	err := a.deployServiceValidator(serviceName, d)
	if err == nil || err.Error() != "service name needs to be at least 3 characters" {
		t.Errorf("Servicename with 2 characters didn't get error message")
	}

	// test with 3 characters
	d = Deploy{
		Containers: []*DeployContainer{
			{
				ContainerName: "abc",
			},
		},
	}
	serviceName = "abc"
	err = a.deployServiceValidator(serviceName, d)
	if err != nil {
		t.Errorf("%v", err)
	}

	// test with wrong container name
	serviceName = "myservice"
	d = Deploy{
		Containers: []*DeployContainer{
			{
				ContainerName: "ab",
			},
			{
				ContainerName: "abd",
			},
		},
	}
	serviceName = "abc"
	err = a.deployServiceValidator(serviceName, d)
	if err == nil {
		t.Errorf("No containerName is equal to serviceName, but no error raised")
	}
}
