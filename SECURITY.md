# Security Policy

## Supported versions

Only `main` is supported. The platform is a reference implementation that runs on a
throwaway kind cluster in CI; nothing is published as a release.

## Reporting a vulnerability

Please report vulnerabilities privately through
[GitHub Security Advisories](https://github.com/ahmedelgmal310/crossplane-self-service/security/advisories/new).
Do not open a public issue. You can expect an acknowledgement within 72 hours.

## Controls in this repository

- Every GitHub Action is pinned to a full commit SHA. Crossplane, provider and function
  packages, the CloudNativePG operator, the PostgreSQL image and the base images are pinned
  by digest. Tools downloaded in CI are checked against a SHA-256.
- Workflows run with `contents: read` and checkout does not persist credentials.
- No credentials exist in the repository. Database passwords are created inside the
  cluster by CloudNativePG and copied into the environment namespace by the platform; the
  only literal password is the throwaway one for the CI service container.
- CI gates: Gitleaks over the full history, govulncheck, golangci-lint (gosec), shellcheck,
  actionlint, hadolint, yamllint, kubeconform, and `crossplane resource validate`.
- Developers get RBAC only for `AppEnvironment` objects in their team namespace. Quotas,
  network policies and database access are created by the platform, not by developers.

## Known trade-off

provider-kubernetes and provider-helm run with `cluster-admin` (see
`platform/providers.yaml`). This is acceptable for a disposable CI cluster and is called out
in the README limitations; a production installation should bind a ClusterRole that covers
only the kinds the Compositions emit.
