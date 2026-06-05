# Security Policy

## Supported Versions

Helix is currently pre-v1. Only the latest commit on the `main` branch is supported for security fixes.

## Reporting a Vulnerability

Please do **not** open a public issue for security vulnerabilities.

Report vulnerabilities through GitHub Security Advisories:

- https://github.com/enokdev/helix/security/advisories/new

If possible, include the following information in your report:

- A clear description of the vulnerability and affected component
- Steps to reproduce or a proof of concept
- Impact assessment, including worst-case outcome
- Suggested mitigation or fix, if known
- Environment details such as OS, Go version, Helix version, and configuration relevant to reproduction

## Response Timeline

- We aim to acknowledge new reports within **7 days**.
- For critical vulnerabilities, we aim to provide a fix or mitigation within **90 days**.
- More complex issues may require additional coordination, validation, or staged disclosure.

## Scope

### In Scope

- Vulnerabilities in Helix framework source code maintained in this repository
- Security issues in first-party modules, starters, CLI features, and examples maintained here
- Issues that could affect confidentiality, integrity, or availability in real Helix deployments

### Out of Scope

- Vulnerabilities in third-party dependencies without a demonstrated Helix-specific impact
- Misconfigurations in user applications or infrastructure outside this repository
- Reports that only concern unsupported commits, forks, or heavily modified downstream distributions
- Denial-of-service findings that require unrealistic resources or non-default environments without practical impact
- Social engineering, phishing, spam, or physical security issues

Thank you for helping keep Helix and its users safe.
