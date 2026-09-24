# Security Policy

## Supported Versions

`go-libs` provides security updates for the current release series (currently v0.3.1):

| Version Series | Status             | Security Updates |
| -------------- | ------------------ | ---------------- |
| 0.3.x          | **Active / Current** | :white_check_mark: Yes |
| 0.2.x          | Maintenance        | :white_check_mark: Yes |
| 0.1.x          | Maintenance        | :white_check_mark: Yes |
| < 0.1.0        | End-of-Life        | :x: No           |

---

## Reporting a Vulnerability

The maintainers of `go-libs` take security and software integrity seriously. If you discover a potential vulnerability, **please do not open a public GitHub issue**. Publicly disclosing a vulnerability could put downstream consumers at risk.

Instead, report vulnerabilities through one of the following confidential channels:

### 1. GitHub Private Vulnerability Reporting (Preferred)
Submit a confidential advisory directly via GitHub:
- Navigate to the **Security** tab of `github.com/Abeta-dev/go-libs`.
- Click **"Report a vulnerability"** to open a private advisory draft.
- Include a description, affected package(s), minimal reproduction or proof-of-concept (PoC), and potential impact.

### 2. Direct Security Contact
If you cannot use GitHub Security Advisories, email the maintainer directly:
- **Email**: [security@abeta.dev](mailto:security@abeta.dev)
- **Subject**: `[SECURITY] go-libs Vulnerability Report: <Package Name>`
- Please include full reproduction steps and environment details.

---

## Response SLA

As a focused maintainer team, we commit to the following response timeline:

- **Initial Acknowledgement**: Within **48 hours** of report receipt.
- **Assessment & Triage**: Within **5 business days**, confirming severity and scope.
- **Fix & Patch Release**: Targeted within **14 business days** depending on complexity.
- **Coordinated Disclosure**: Coordinated with the reporter via a GitHub Security Advisory and published alongside a patch release.

