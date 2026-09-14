package webrpc

import "testing"

func TestAllowedFrontendMethods(t *testing.T) {
	for _, name := range []string{"GetConfig", "UpdateConfig", "GetVersionInfo", "ImportSkillFromBase64", "ImportTradingRecordsFromPath"} {
		if !Allowed(name) {
			t.Fatalf("expected %s to be allowed", name)
		}
	}
}

func TestRejectedInternalMethods(t *testing.T) {
	for _, name := range []string{"", "startup", "shutdown", "downloadUpdate", "isVip", "UnknownMethod"} {
		if Allowed(name) {
			t.Fatalf("did not expect %s to be allowed", name)
		}
	}
}
