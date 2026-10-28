# Pinned versions

Every package, chart, image, tool and action is pinned. Crossplane packages and images use
`tag@sha256:digest`, actions use a full commit SHA with the version in a comment, downloaded
binaries are checked against a SHA-256, and Go modules are locked by `go.sum`. A version is
adopted only after it has been public for **at least 14 days**; Dependabot enforces the same
cooldown (`.github/dependabot.yml`). Versions were checked on 2026-10-28.

Where each pin lives: platform versions in `e2e/lib.sh`, packages in `platform/*.yaml`,
the database chart and image in `apis/appenvironment/composition-kind.yaml`, render and CI
tools in `.github/workflows/ci.yml` and `scripts/render-test.sh`, build images in
`Dockerfile`, Go modules in `go.mod`. Update this file in the same change.

## Exceptions to the 14-day rule

**Crossplane v2.4.3** (released 2026-10-22, 6 days before the check). It is a security
release: Go 1.26.10 with the `crypto/x509` fix that distrusts the compromised eMudhra
intermediate CAs (CVE-2026-111943), removal of the affected roots from the image CA bundle,
and a batch of dependency CVE fixes. v2.4.2 (2026-09-22) passes the cooldown but ships the
vulnerable toolchain and CA bundle, and Crossplane pulls packages over TLS from public
registries, which is exactly the code path that fix protects. The Helm chart and the image
used by `crossplane composition render` / `crossplane resource validate` are the same
v2.4.3.

**Go 1.27.3** (released 2026-10-19) for the same reason as in my other repositories: it
carries `crypto/x509` security fixes and govulncheck reports the vulnerability as reachable
from code that opens TLS connections (pgx to PostgreSQL). 1.27.2 passes the cooldown but
is vulnerable.

These are the only exceptions.

## Control plane (`e2e/lib.sh`, `platform/`)

| Component | Version | Released | Pin |
|---|---|---|---|
| Crossplane (Helm chart + core image) | v2.4.3 | 2026-10-22 (security exception) | `xpkg.crossplane.io/crossplane/crossplane@sha256:1c66261d805a73cade46e2ac94b665c73aebd4a00844de5fcae4bcadaa2be638` |
| provider-kubernetes | v1.4.1 | 2026-10-08 | `sha256:19bac959b88d49fee4f1b2cc76a085210a4663aff04a9d1a307ac7339e2ae82a` |
| provider-helm | v1.4.0 | 2026-08-11 | `sha256:2280e32fd4e42f0bbe66d1dcd8ddd9cc3a4d46eaba9f4bccddfaec2302664926` (v1.5.0 of 2026-10-15 is inside the cooldown) |
| function-patch-and-transform | v0.11.0 | 2026-09-25 | `sha256:677ad7d06eab2da219c9f28a7f6393fd74e5cfe445b98e324a23736163fb4e99` |
| function-cel-filter | v0.2.0 | 2026-03-22 | `sha256:d5c77d75131b2cfdb2df013cc62c58e2e73fc3add947c3798a2963b16322f35f` |
| CloudNativePG operator chart (operator 1.30.1) | 0.29.1 | 2026-09-23 | chart version; operator image `ghcr.io/cloudnative-pg/cloudnative-pg@sha256:923c267ec29636db3bee20f993d0ec4973fa22998e1adad37da79e4d32b5bc07` |
| CloudNativePG `cluster` chart (per environment) | 0.9.0 | 2026-10-01 | chart version in the Composition |
| PostgreSQL | 18.6 (`minimal-trixie`, the operator 1.30.1 default line) | before 2026-09-23 | `ghcr.io/cloudnative-pg/postgresql@sha256:64abc98a4387361afd6821a3a99f975763bbd87bbdde9449e75ad6002528a318` |
| kind | v0.33.0 | 2026-08-26 | `helm/kind-action` input |
| Kubernetes (kind node) | v1.36.4 | 2026-08-20 | `kindest/node@sha256:099e049362a1526b2db71494e1947aae99bd16290d7c895f2b7ea312e3cbfaed` |
| kubectl | v1.36.4 | 2026-08-20 | `helm/kind-action` input |
| Helm | v4.3.0 | 2026-09-09 | `azure/setup-helm` input |

## Schemas used only for validation (`test/schemas/extensions.yaml`)

| Package | Version | Released |
|---|---|---|
| crossplane-contrib/provider-aws-rds | v2.8.1 | 2026-09-21 |
| crossplane-contrib/provider-aws-s3 | v2.8.1 | 2026-09-21 |

These are downloaded by `crossplane resource validate` to check the render-only AWS
Composition. They are never installed and no AWS credentials exist anywhere in the project.

## Workload and CLI (`Dockerfile`, `go.mod`)

| Component | Version | Pin |
|---|---|---|
| golang (build stage) | 1.27.3-alpine3.24 | `sha256:d200d2dd76b03a99b52651fd1325aff7dcd35422c35c01bac56571b0507f3deb` |
| distroless static-debian13:nonroot (runtime) | | `sha256:18f4cc26c9ffa7f20a359298be7deadac5c607c8e03a02b8119f1a5fc2f82c99` |
| github.com/jackc/pgx/v5 | v5.11.0 (2026-09-07) | `go.sum` |
| golang.org/x/text | v0.43.0 (2026-10-09) | `go.sum` |
| golang.org/x/sync | v0.24.0 (2026-09-23) | `go.sum` |
| PostgreSQL service container for Go integration tests | 18.6-alpine3.24 | `postgres@sha256:10a5c69a83035ab5239f5620d708520e8b28f5aa1f2b20143e6024532edadfb5` |

## CI (`.github/workflows/ci.yml`)

| Tool | Version | Released | Pin |
|---|---|---|---|
| actions/checkout | v7.0.1 | 2026-07-20 | `3d3c42e5aac5ba805825da76410c181273ba90b1` |
| actions/setup-go | v7.0.0 | 2026-07-16 | `b7ad1dad31e06c5925ef5d2fc7ad053ef454303e` |
| actions/upload-artifact | v7.0.2 | 2026-10-07 | `cf430e030ddbb5b0abf93d22962f4752f3646cd9` |
| golangci/golangci-lint-action | v9.3.0 (golangci-lint v2.14.0, 2026-09-24) | 2026-06-29 | `ba0d7d2ec06a0ea1cb5fa41b2e4a3ab91d21278a` |
| helm/kind-action | v1.15.1 | 2026-10-06 | `1544676c07bb3570fcb70166beaa07c4528fd4aa` |
| azure/setup-helm | v5.0.1 | 2026-06-23 | `9bc31f4ebc9c6b171d7bfbaa5d006ae7abdb4310` |
| docker/setup-buildx-action | v4.4.1 | 2026-09-16 | `f87e5991a6d7451dcb8d9637bfbc97413f497069` |
| docker/build-push-action | v7.4.0 | 2026-09-15 | `c3c9e263c25d99ce0380d002d59b67737d91b0dc` |
| crossplane CLI | v2.5.0 | 2026-08-21 | SHA-256 `065dd6b40c9ea5fc71e49900d5c924cc0bec7f58375922ddbde6b402f292347e` |
| kubeconform | v0.8.0 | 2026-06-04 | SHA-256 `9bc2bffbf71f261128533edaf912153948b7ff238f9a531ae6d34466ec287883` |
| govulncheck | v1.9.0 | 2026-10-11 | module version |
| Gitleaks | v8.30.1 | 2026-03-21 | `sha256:c00b6bd0aeb3071cbcb79009cb16a60dd9e0a7c60e2be9ab65d25e6bc8abbb7f` |
| actionlint | 1.7.12 | 2026-03-30 | `sha256:b1934ee5f1c509618f2508e6eb47ee0d3520686341fec936f3b79331f9315667` |
| hadolint | v2.15.1 | 2026-07-31 | `sha256:32dac94127fd60b7b7e3fbfc65e1383b9b5e25c9bfd7b8536de7a539fe68a12d` |
| yamllint | 1.38.0 | 2026-01-13 | PyPI version via `pipx run` |
| shellcheck | preinstalled on the runner image | | |
