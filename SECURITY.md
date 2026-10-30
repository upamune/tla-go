# Security

## Report a vulnerability

Use [GitHub private vulnerability reporting](https://github.com/upamune/tla-go/security/advisories/new). Include the affected version, a minimal reproduction using synthetic data, and the impact. Do not post working exploits or sensitive data in a public issue.

The implementation is still incomplete; do not rely on its model-checking results as a replacement for Java TLC without independent verification.

## Continuous checks

The repository runs CodeQL, govulncheck, OSV-Scanner, dependency review, secret scanning, zizmor, OpenSSF Scorecard, and parser fuzzing. Dependabot monitors Go modules and GitHub Actions. Successful checks do not guarantee the absence of vulnerabilities.
