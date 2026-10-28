// Command platformctl scaffolds AppEnvironment claims.
//
//	platformctl new --name checkout-api --team payments --size medium \
//	  --database --exposure public --hostname checkout.example.test \
//	  --image ghcr.io/acme/checkout:1.4.2 | kubectl apply -f -
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ahmedelgmal310/crossplane-self-service/internal/claim"
)

var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

const usage = `platformctl scaffolds AppEnvironment claims for the self-service platform.

Usage:
  platformctl new [flags]    print an AppEnvironment manifest
  platformctl version        print the version

Run "platformctl new -h" for the flags.
`

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	switch args[0] {
	case "new":
		return runNew(args[1:], stdout, stderr)
	case "version":
		fmt.Fprintln(stdout, version)
		return 0
	case "-h", "--help", "help":
		fmt.Fprint(stdout, usage)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n%s", args[0], usage)
		return 2
	}
}

func runNew(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("new", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var s claim.Spec
	fs.StringVar(&s.Name, "name", "", "environment name (DNS label)")
	fs.StringVar(&s.Team, "team", "", "owning team; the claim goes to namespace team-<team>")
	fs.StringVar(&s.Size, "size", "small", "size: "+strings.Join(claim.Sizes, "|"))
	fs.BoolVar(&s.Database, "database", false, "provision PostgreSQL and deliver credentials as Secret app-db")
	fs.StringVar(&s.Exposure, "exposure", "internal", "exposure: "+strings.Join(claim.Exposures, "|"))
	fs.StringVar(&s.Hostname, "hostname", "", "ingress host, required for --exposure public")
	fs.StringVar(&s.Image, "image", "", "application container image")
	fs.IntVar(&s.Port, "port", 8080, "container port")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "unexpected arguments: %v\n", fs.Args())
		return 2
	}
	if err := claim.Render(stdout, s); err != nil {
		fmt.Fprintf(stderr, "invalid claim:\n  %s\n", strings.ReplaceAll(err.Error(), "\n", "\n  "))
		return 1
	}
	return 0
}
