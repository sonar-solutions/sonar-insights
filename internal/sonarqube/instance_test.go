package sonarqube

import (
	"strings"
	"testing"
)

func TestAuthorizationHeader_Cloud(t *testing.T) {
	inst := SonarInstance{Product: Cloud, Token: "mytoken"}
	if got := inst.AuthorizationHeader(); got != "Bearer mytoken" {
		t.Errorf("expected Bearer token, got %q", got)
	}
}

func TestAuthorizationHeader_ServerBearer(t *testing.T) {
	for _, version := range []string{"10.2.0.100512", "10.9.0.100512", "11.0.0.100512"} {
		inst := SonarInstance{Product: Server, Version: version, Token: "tok"}
		if got := inst.AuthorizationHeader(); !strings.HasPrefix(got, "Bearer ") {
			t.Errorf("version %s: expected Bearer, got %q", version, got)
		}
	}
}

func TestAuthorizationHeader_ServerBasic(t *testing.T) {
	for _, version := range []string{"10.1.0.100512", "9.9.6.100512", "8.0.0.0"} {
		inst := SonarInstance{Product: Server, Version: version, Token: "tok"}
		if got := inst.AuthorizationHeader(); !strings.HasPrefix(got, "Basic ") {
			t.Errorf("version %s: expected Basic, got %q", version, got)
		}
	}
}

func TestAuthorizationHeader_BasicCredentials(t *testing.T) {
	// Basic auth: base64("token:") — token as username, empty password
	inst := SonarInstance{Product: Server, Version: "9.9.0.0", Token: "mytoken"}
	got := inst.AuthorizationHeader()
	// "bXl0b2tlbjo=" is base64("mytoken:")
	if got != "Basic bXl0b2tlbjo=" {
		t.Errorf("unexpected basic auth header: %q", got)
	}
}
