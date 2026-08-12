package resources

import "regexp"

// Shared compiled regexps used by schema validators across all resources.
// Compiled once at package init, reused on every plan.

// URLRegexp matches http:// or https:// URLs.
var URLRegexp = regexp.MustCompile(`^https?://[a-zA-Z0-9._\-]+(:[0-9]+)?(/.*)?$`)

// LabelRegexp matches IBM Verify signer cert labels:
// 1–128 characters of letters, digits, dots, hyphens, or underscores.
// This also prevents path-traversal attacks via the label in API calls.
var LabelRegexp = regexp.MustCompile(`^[A-Za-z0-9._\-]{1,128}$`)

// PEMCertRegexp checks that a string starts with the PEM certificate header.
var PEMCertRegexp = regexp.MustCompile(`(?s)^\s*-----BEGIN CERTIFICATE-----`)
