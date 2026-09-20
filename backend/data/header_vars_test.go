package data

import (
	"testing"

	"go-stock/backend/webmode"
)

func TestExpandHeaderVarsSkipsEnvInWebMode(t *testing.T) {
	t.Setenv("WEB_ADMIN_PASSWORD", "super-secret")
	webmode.Enable()
	t.Cleanup(webmode.Disable)

	got := ExpandHeaderVars("token={{env.WEB_ADMIN_PASSWORD}}", "")
	if got != "token={{env.WEB_ADMIN_PASSWORD}}" {
		t.Fatalf("web mode leaked env: %q", got)
	}
}

func TestExpandHeaderVarsExpandsEnvOnDesktop(t *testing.T) {
	t.Setenv("WESTOCK_TOKEN", "abc")
	webmode.Disable()
	got := ExpandHeaderVars("token={{env.WESTOCK_TOKEN}}", "")
	if got != "token=abc" {
		t.Fatalf("desktop env expand: %q", got)
	}
}
