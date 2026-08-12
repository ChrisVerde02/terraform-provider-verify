// Package tests contains black-box tests for the resources package validators.
package tests

import (
	"testing"

	"github.com/Christian-Verderame/terraform-provider-verify/internal/resources"
)

func TestURLRegexp(t *testing.T) {
	valid := []string{
		"https://example.verify.ibm.com",
		"https://example.verify.ibm.com/",
		"http://localhost:8080",
		"https://tenant.verify.ibm.com/some/path",
	}
	for _, u := range valid {
		if !resources.URLRegexp.MatchString(u) {
			t.Errorf("URLRegexp should match %q", u)
		}
	}

	invalid := []string{
		"banana",
		"ftp://example.com",
		"example.com",
		"",
	}
	for _, u := range invalid {
		if resources.URLRegexp.MatchString(u) {
			t.Errorf("URLRegexp should not match %q", u)
		}
	}
}

func TestLabelRegexp(t *testing.T) {
	valid := []string{
		"demotokensigner",
		"demo-token-signer",
		"demo.token.signer",
		"demo_token_signer",
		"abc123",
		"a",
	}
	for _, l := range valid {
		if !resources.LabelRegexp.MatchString(l) {
			t.Errorf("LabelRegexp should match %q", l)
		}
	}

	invalid := []string{
		"",
		"../etc/passwd",
		"label with spaces",
		"label/slash",
		"label@at",
	}
	for _, l := range invalid {
		if resources.LabelRegexp.MatchString(l) {
			t.Errorf("LabelRegexp should not match %q", l)
		}
	}

	// Exactly 128 chars — valid
	long128 := ""
	for i := 0; i < 128; i++ {
		long128 += "a"
	}
	if !resources.LabelRegexp.MatchString(long128) {
		t.Error("128-char label should be valid")
	}
	if resources.LabelRegexp.MatchString(long128 + "a") {
		t.Error("129-char label should be invalid")
	}
}

func TestPEMCertRegexp(t *testing.T) {
	valid := []string{
		"-----BEGIN CERTIFICATE-----\nMIIF...\n-----END CERTIFICATE-----",
		"\n-----BEGIN CERTIFICATE-----\nMIIF",
		"  -----BEGIN CERTIFICATE-----",
	}
	for _, p := range valid {
		if !resources.PEMCertRegexp.MatchString(p) {
			t.Errorf("PEMCertRegexp should match %q", p)
		}
	}

	invalid := []string{
		"",
		"not a pem",
		"-----BEGIN PRIVATE KEY-----",
		"-----BEGIN RSA PRIVATE KEY-----",
	}
	for _, p := range invalid {
		if resources.PEMCertRegexp.MatchString(p) {
			t.Errorf("PEMCertRegexp should not match %q", p)
		}
	}
}
