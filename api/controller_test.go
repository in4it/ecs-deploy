package api

import (
	"github.com/in4it/ecs-deploy/service"
)

type MockController struct {
	ControllerIf
	runningServices []service.RunningService
}

func (m *MockController) describeServices() ([]service.RunningService, error) {
	return m.runningServices, nil
}
