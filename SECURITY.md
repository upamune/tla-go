# Security

## Report a vulnerability

Use [GitHub private vulnerability reporting](https://github.com/upamune/tla-go/security/advisories/new). Include the affected version, a minimal reproduction using synthetic data, and the impact. Do not post working exploits or sensitive data in a public issue.

The implementation is still incomplete; do not rely on its model-checking results as a replacement for Java TLC without independent verification.

## Continuous checks

| Check | Status | Coverage |
| --- | --- | --- |
| CodeQL | [![CodeQL](https://github.com/upamune/tla-go/actions/workflows/codeql.yml/badge.svg?branch=main)](https://github.com/upamune/tla-go/actions/workflows/codeql.yml) | Static analysis of Go code. |
| govulncheck | [![Dependency audit](https://github.com/upamune/tla-go/actions/workflows/dependency-audit.yml/badge.svg?branch=main)](https://github.com/upamune/tla-go/actions/workflows/dependency-audit.yml) | Known vulnerabilities in Go dependencies. |
| OSV-Scanner | [![OSV-Scanner](https://github.com/upamune/tla-go/actions/workflows/osv.yml/badge.svg?branch=main)](https://github.com/upamune/tla-go/actions/workflows/osv.yml) | Known vulnerabilities in dependencies. |
| Dependency review | [![Dependency review](https://github.com/upamune/tla-go/actions/workflows/dependency-review.yml/badge.svg?branch=main)](https://github.com/upamune/tla-go/actions/workflows/dependency-review.yml) | Dependency changes in pull requests. |
| Secret scanning | [![CI](https://github.com/upamune/tla-go/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/upamune/tla-go/actions/workflows/ci.yml) | Gitleaks runs as part of CI. |
| zizmor | [![Workflow security](https://github.com/upamune/tla-go/actions/workflows/zizmor.yml/badge.svg?branch=main)](https://github.com/upamune/tla-go/actions/workflows/zizmor.yml) | GitHub Actions workflow security. |
| OpenSSF Scorecard | [![OpenSSF Scorecard](https://github.com/upamune/tla-go/actions/workflows/scorecard.yml/badge.svg?branch=main)](https://github.com/upamune/tla-go/actions/workflows/scorecard.yml) | Repository security practices. |
| Parser fuzzing | [![Parser fuzzing](https://github.com/upamune/tla-go/actions/workflows/fuzz.yml/badge.svg?branch=main)](https://github.com/upamune/tla-go/actions/workflows/fuzz.yml) | Parser testing with generated inputs. |

Badges show the latest workflow status on `main`. Dependency review runs on pull requests or by manual dispatch, so its badge may show “no status”; check individual pull requests for their results. Secret scanning shares the CI workflow badge. Dependabot monitors Go modules and GitHub Actions. Successful checks do not guarantee the absence of vulnerabilities.
