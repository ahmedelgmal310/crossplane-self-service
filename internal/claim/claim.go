// Package claim builds AppEnvironment manifests for platformctl. It checks the
// rules a developer can get wrong offline (names, enums, hostname for public
// exposure) so mistakes surface before kubectl apply. Team limits are not
// checked here: the API server's admission policy is the source of truth.
package claim

import (
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"text/template"
)

// Spec is everything a developer chooses for an environment.
type Spec struct {
	Name     string
	Team     string
	Size     string
	Database bool
	Exposure string
	Hostname string
	Image    string
	Port     int
}

var (
	dnsLabel    = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
	dnsHostname = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)+$`)
)

// Sizes and Exposures are the values the AppEnvironment schema accepts.
var (
	Sizes     = []string{"small", "medium"}
	Exposures = []string{"internal", "public"}
)

// Namespace is the team namespace the claim is created in.
func (s Spec) Namespace() string { return "team-" + s.Team }

// EnvironmentNamespace is the namespace the platform will create.
func (s Spec) EnvironmentNamespace() string { return s.Team + "-" + s.Name }

// Validate returns every rule the spec breaks, joined.
func (s Spec) Validate() error {
	var errs []error
	if !dnsLabel.MatchString(s.Name) {
		errs = append(errs, fmt.Errorf("name %q must be a lowercase DNS label", s.Name))
	}
	if !dnsLabel.MatchString(s.Team) {
		errs = append(errs, fmt.Errorf("team %q must be a lowercase DNS label", s.Team))
	}
	if n := len(s.EnvironmentNamespace()); n > 63 {
		errs = append(errs, fmt.Errorf("team and name give namespace %q of %d characters; the limit is 63", s.EnvironmentNamespace(), n))
	}
	if !contains(Sizes, s.Size) {
		errs = append(errs, fmt.Errorf("size %q must be one of %s", s.Size, strings.Join(Sizes, ", ")))
	}
	if !contains(Exposures, s.Exposure) {
		errs = append(errs, fmt.Errorf("exposure %q must be one of %s", s.Exposure, strings.Join(Exposures, ", ")))
	}
	if s.Exposure == "public" && s.Hostname == "" {
		errs = append(errs, errors.New("hostname is required when exposure is public"))
	}
	if s.Hostname != "" && !dnsHostname.MatchString(s.Hostname) {
		errs = append(errs, fmt.Errorf("hostname %q is not a valid DNS name", s.Hostname))
	}
	if s.Image == "" {
		errs = append(errs, errors.New("image is required"))
	}
	if s.Port < 1024 || s.Port > 65535 {
		errs = append(errs, fmt.Errorf("port %d must be between 1024 and 65535", s.Port))
	}
	return errors.Join(errs...)
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

var manifest = template.Must(template.New("claim").Parse(`apiVersion: platform.example.io/v1alpha1
kind: AppEnvironment
metadata:
  name: {{ .Name }}
  namespace: {{ .Namespace }}
  labels:
    platform.example.io/team: {{ .Team }}
spec:
  size: {{ .Size }}
  database: {{ .Database }}
  exposure: {{ .Exposure }}
{{- if .Hostname }}
  hostname: {{ .Hostname }}
{{- end }}
  image: {{ printf "%q" .Image }}
  port: {{ .Port }}
`))

// Render validates the spec and writes the AppEnvironment manifest.
func Render(w io.Writer, s Spec) error {
	if err := s.Validate(); err != nil {
		return err
	}
	return manifest.Execute(w, s)
}
