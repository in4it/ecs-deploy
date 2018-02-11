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
	if p.getPrefix() != "/mycompany-staging/ecs-deploy/" {
		t.Errorf("Wrong prefix returned: %v", p.getPrefix())
	}
	os.Setenv("AWS_ENV_PATH", "")
	if p.getPrefix() != "/mycompany2-prod/ecs-deploy/" {
		t.Errorf("Wrong prefix returned: %v", p.getPrefix())
	}
}
