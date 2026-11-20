package version

import "testing"

func TestCurrentPrefersEnvironmentThenBuildValue(t *testing.T) {
	original := Version
	defer func() { Version = original; t.Setenv(EnvVar, "") }()

	Version = "0.1.0"
	t.Setenv(EnvVar, "")
	if got := Current(); got != "0.1.0" {
		t.Fatalf("Current() = %q, want the linked build version", got)
	}

	t.Setenv(EnvVar, "0.2.0-rc.1")
	if got := Current(); got != "0.2.0-rc.1" {
		t.Fatalf("Current() = %q, want the %s override", got, EnvVar)
	}

	t.Setenv(EnvVar, "")
	Version = ""
	if got := Current(); got != "dev" {
		t.Fatalf("Current() = %q, want fallback dev", got)
	}
}
