# Security Policy

## Reporting a Vulnerability

**Please do NOT open a public GitHub issue for security vulnerabilities.**
Public disclosure before a fix is ready puts all Aethium users at risk.

Instead, report vulnerabilities privately by emailing:

**admin.forestritium@gmail.com**

Include the following in your report:
- Description of the vulnerability
- Steps to reproduce it
- The version of Aethium affected
- Potential impact
- Your suggested fix if you have one

You will receive a response within a short time acknowledging your report.

## What qualifies as a security vulnerability

- Sandbox escape — an `.aeth` script bypassing `AllowSubprocess` or
  `AllowNetwork` config flags
- Memory corruption or buffer overflows in the VM
- Arbitrary code execution via malformed bytecode or source input
- Path traversal in the module import system
- FFI registry abuse allowing unintended native code execution

## What does not qualify

- Bugs that require the attacker to already have arbitrary code execution
- Performance issues or crashes without security impact
- Issues in user-written `.aeth` scripts themselves

## Disclosure policy

Once a fix is ready and released, I will publish a security advisory on
GitHub describing the vulnerability, its impact, and the fix. Credit will
be given to the reporter unless they prefer to remain anonymous.