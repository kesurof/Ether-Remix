package admin_test

import (
	"testing"

	"github.com/braedonsaunders/ether/internal/admin"
	"github.com/braedonsaunders/ether/internal/config"
)

func TestVerifiedFilesValidation(t *testing.T) {
	if got := config.Defaults(t.TempDir()).Resolver.VerifiedFiles; got != "first" {
		t.Fatalf("défaut verifiedFiles = %q, attendu first", got)
	}
	for _, tc := range []struct {
		value string
		valid bool
	}{{"", true}, {"off", true}, {"tie", true}, {"first", true}, {"always", false}, {"FIRST", false}} {
		cfg := config.Defaults(t.TempDir())
		cfg.Resolver.VerifiedFiles = tc.value
		rejected := false
		for _, e := range admin.ValidateConfig(cfg) {
			if e.Field == "resolver.verifiedFiles" {
				rejected = true
			}
		}
		if rejected == tc.valid {
			t.Errorf("verifiedFiles %q : rejeté=%v, attendu valide=%v", tc.value, rejected, tc.valid)
		}
	}
}

func TestFailoverAttemptsValidation(t *testing.T) {
	if got := config.Defaults(t.TempDir()).Resolver.FailoverAttempts; got != 3 {
		t.Fatalf("défaut failoverAttempts = %d, attendu 3", got)
	}
	for _, tc := range []struct {
		value int
		valid bool
	}{{0, true}, {1, true}, {3, true}, {10, true}, {-1, false}, {11, false}} {
		cfg := config.Defaults(t.TempDir())
		cfg.Resolver.FailoverAttempts = tc.value
		rejected := false
		for _, e := range admin.ValidateConfig(cfg) {
			if e.Field == "resolver.failoverAttempts" {
				rejected = true
			}
		}
		if rejected == tc.valid {
			t.Errorf("failoverAttempts %d : rejeté=%v, attendu valide=%v", tc.value, rejected, tc.valid)
		}
	}
}
