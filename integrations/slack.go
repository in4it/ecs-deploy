package integrations

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/in4it/ecs-deploy/util"
	"github.com/juju/loggo"
)

var slackLogger = loggo.GetLogger("integrations.slack")

type payload struct {
	Text      string `json:"text"`
	Username  string `json:"username,omitempty"`
	Channel   string `json:"channel,omitempty"`
	IconEmoji string `json:"icon_emoji"`
}

type Slack struct {
}

func NewSlack() *Slack {
	return &Slack{}
}

func (s *Slack) LogFailure(message string) error {
	return s.sendMsg(message, "failure")
}

func (s *Slack) LogRecovery(message string) error {
	return s.sendMsg(message, "recovery")
}

func (s *Slack) sendMsg(message, status string) error {
	if util.GetEnv("SLACK_WEBHOOKS", "") == "" {
		return fmt.Errorf("SLACK_WEBHOOKS not set")
	}

	username := util.GetEnv("SLACK_USERNAME", "ecs-deploy")

	// add environment
	if util.GetEnv("AWS_ACCOUNT_ENV", "") != "" {
		message = "[" + util.GetEnv("AWS_ACCOUNT_ENV", "") + "] " + message
	}

	icon := ":vertical_traffic_light:"
	webhooks := strings.Split(util.GetEnv("SLACK_WEBHOOKS", ""), ",")

	for _, v := range webhooks {
		webhook := strings.Split(v, ":#")
		channel := ""

		if len(webhook) > 1 {
			channel = "#" + webhook[1]
		}

		payload := &payload{
			Text:      message,
			IconEmoji: icon,
			Username:  username,
			Channel:   channel,
		}
		payloadJSON, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		buf := bytes.NewBuffer(payloadJSON)
		if err != nil {
			return err
		}
		slackLogger.Debugf("Sending slack notification: %v", buf)
		resp, err := http.Post(webhook[0], "application/json", buf)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if string(body) != "ok" {
			return fmt.Errorf("Wrong response: %s", string(body))
		}
		return nil
	}

	return nil
}
