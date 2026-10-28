package claim

import (
	"bytes"
	"strings"
	"testing"
)

func valid() Spec {
	return Spec{
		Name:     "checkout-api",
		Team:     "payments",
		Size:     "medium",
		Database: true,
		Exposure: "public",
		Hostname: "checkout.payments.apps.example.test",
		Image:    "env-probe:e2e",
		Port:     8080,
	}
}

func TestRenderValid(t *testing.T) {
	var buf bytes.Buffer
	if err := Render(&buf, valid()); err != nil {
		t.Fatalf("Render: %v", err)
	}
	want := `apiVersion: platform.example.io/v1alpha1
kind: AppEnvironment
metadata:
  name: checkout-api
  namespace: team-payments
  labels:
    platform.example.io/team: payments
spec:
  size: medium
  database: true
  exposure: public
  hostname: checkout.payments.apps.example.test
  image: "env-probe:e2e"
  port: 8080
`
	if got := buf.String(); got != want {
		t.Errorf("manifest mismatch\n--- got\n%s\n--- want\n%s", got, want)
	}
}

func TestRenderOmitsEmptyHostname(t *testing.T) {
	s := valid()
	s.Exposure, s.Hostname, s.Database, s.Size = "internal", "", false, "small"
	var buf bytes.Buffer
	if err := Render(&buf, s); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if strings.Contains(buf.String(), "hostname") {
		t.Errorf("hostname should be omitted:\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), "database: false") {
		t.Errorf("database flag missing:\n%s", buf.String())
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Spec)
		want   string
	}{
		{"bad name", func(s *Spec) { s.Name = "Checkout_API" }, "name"},
		{"bad team", func(s *Spec) { s.Team = "" }, "team"},
		{"too long", func(s *Spec) { s.Name = strings.Repeat("a", 60) }, "limit is 63"},
		{"bad size", func(s *Spec) { s.Size = "xlarge" }, "size"},
		{"bad exposure", func(s *Spec) { s.Exposure = "world" }, "exposure"},
		{"public needs hostname", func(s *Spec) { s.Hostname = "" }, "hostname is required"},
		{"bad hostname", func(s *Spec) { s.Hostname = "not a host" }, "not a valid DNS name"},
		{"missing image", func(s *Spec) { s.Image = "" }, "image is required"},
		{"low port", func(s *Spec) { s.Port = 80 }, "port"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := valid()
			tc.mutate(&s)
			err := s.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Validate() = %v, want error containing %q", err, tc.want)
			}
			var buf bytes.Buffer
			if Render(&buf, s) == nil || buf.Len() != 0 {
				t.Error("Render must refuse an invalid spec and write nothing")
			}
		})
	}
}

func TestNamespaces(t *testing.T) {
	s := valid()
	if s.Namespace() != "team-payments" || s.EnvironmentNamespace() != "payments-checkout-api" {
		t.Errorf("got %q and %q", s.Namespace(), s.EnvironmentNamespace())
	}
}
