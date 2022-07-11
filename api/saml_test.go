package api

import (
	"net/url"
	"testing"
)

func TestSAML(t *testing.T) {
	ecsURL := "https://localhost/ecs-deploy/"
	rootURL, err := url.Parse(ecsURL)
	if err != nil {
		t.Errorf("error: %s", err)
	}

	acsURL := rootURL.ResolveReference(&url.URL{Path: "saml/acs"})

	if acsURL.Path != "/ecs-deploy/saml/acs" {
		t.Errorf("Got wrong url: %s", acsURL.Path)
	}
}
