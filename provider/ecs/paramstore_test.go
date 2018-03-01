package ecs

import (
	"os"
	"testing"
)

func TestGetPrefix(t *testing.T) {
	p := Paramstore{}
	os.Setenv("AWS_ENV_PATH", "/mycompany-staging/ecs-deploy/")
	os.Setenv("PARAMSTORE_PREFIX", "mycompany2")
	os.Setenv("AWS_ACCOUNT_ENV", "prod")
	if p.GetPrefix() != "/mycompany-staging/ecs-deploy/" {
		t.Errorf("Wrong prefix returned: %v", p.GetPrefix())
	}
	os.Setenv("AWS_ENV_PATH", "")
	if p.GetPrefix() != "/mycompany2-prod/ecs-deploy/" {
		t.Errorf("Wrong prefix returned: %v", p.GetPrefix())
	}
}
