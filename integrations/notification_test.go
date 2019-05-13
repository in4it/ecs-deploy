package integrations_test

import (
	"strings"
	"testing"

	"github.com/in4it/ecs-deploy/integrations"
)

func TestDummyIntegration(t *testing.T) {
	var notification integrations.Notification
	notification = integrations.NewDummy()
	err := notification.LogRecovery("Deployed successfully")
	if err != nil {
		t.Errorf("Could not send notification: %s", err)
	}

}
func TestSlackIntegration(t *testing.T) {
	var notification integrations.Notification
	notification = integrations.NewSlack()
	err := notification.LogRecovery("Deployed successfully")
	if err != nil && !strings.HasSuffix(err.Error(), "SLACK_WEBHOOKS not set") {
		t.Errorf("Could not send notification: %s", err)
	}
}
