package api

import (
	"testing"

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

func TestDefaultTemplate(t *testing.T) {
	_, err := defaultTemplates.ReadFile("default-templates/ecs-deploy-task.json")
	if err != nil {
		t.Errorf("could not read default template ecs-deploy-task.json: %s", err)
	}
}
