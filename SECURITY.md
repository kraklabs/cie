# Security Policy

## Supported Versions

We release security patches for the following versions:

| Version | Supported          |
| ------- | ------------------ |
| 0.x.x   | :white_check_mark: |

As CIE is pre-1.0, we currently support only the latest minor release.
Once we reach 1.0, we will maintain security support for the current
and previous minor versions.

## Reporting a Vulnerability

We take security vulnerabilities seriously. If you discover a security
issue, please report it responsibly.

### How to Report

**Please do NOT report security vulnerabilities through public GitHub issues.**

Instead, please use one of these methods:

1. **GitHub Security Advisories** (Preferred)
   - Go to the [Security tab](https://github.com/kraklabs/cie/security/advisories)
   - Click "New draft security advisory"
   - Fill in the details

2. **Email**
   - Send details to: security@kraklabs.com
   - Include "CIE Security" in the subject line

### What to Include

Please include as much of the following information as possible:

- Type of vulnerability (e.g., buffer overflow, SQL injection, XSS)
- Full paths of affected source files
- Location of the affected code (tag/branch/commit or direct URL)
- Step-by-step instructions to reproduce the issue
- Proof-of-concept or exploit code (if possible)
- Impact assessment and potential severity

### What to Expect

- **Acknowledgment**: Within 48 hours of your report
- **Initial Assessment**: Within 7 days
- **Resolution Timeline**: Depends on severity
  - Critical: Target fix within 7 days
  - High: Target fix within 30 days
  - Medium/Low: Target fix in next release

We will keep you informed of our progress and may ask for additional
information or guidance.

### Disclosure Policy

- We follow coordinated disclosure practices
- We will credit reporters in security advisories (unless you prefer anonymity)
- We ask that you give us reasonable time to address the issue before public disclosure

## Security Best Practices

When using CIE:

- Keep CIE updated to the latest version
- Review indexed repositories for sensitive data before sharing
- Use environment variables for API keys, never hardcode them
- Run CIE in containers or sandboxed environments when indexing untrusted code

## Past Security Advisories

No security advisories have been published yet.

---

Thank you for helping keep CIE and its users safe!