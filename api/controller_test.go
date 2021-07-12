package api

import (
	"github.com/in4it/ecs-deploy/service"
)

type MockController struct {
	ControllerIf
	runningServices   []service.RunningService
	getServicesOutput []*service.DynamoServicesElement
}

func (m *MockController) getServices() ([]*service.DynamoServicesElement, error) {
	return m.getServicesOutput, nil
}
func (m *MockController) describeServices() ([]service.RunningService, error) {
	return m.runningServices, nil
}
