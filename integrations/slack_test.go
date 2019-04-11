package integrations

import "testing"

func TestSendMsg(t *testing.T) {
	slack := NewSlack()
	err := slack.sendMsg("test message", "failure")

	if err != nil {
		if err.Error() == "SLACK_WEBHOOKS not set" {
			t.Skipf("Skipped: %s", err)
		}
		t.Errorf("Error: %s", err)
	}
}
