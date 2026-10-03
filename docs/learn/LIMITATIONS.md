# Groot IAM - Known Limitations

This file is updated at the end of every phase. It records what is
simulated, partial, or not the real thing. Only what is built and
passing tests goes on the resume.

## Phase 0 (Recon and Plan)

| Item | What we have | What it is NOT |
|------|-------------|----------------|
| OpenLDAP | Open-source LDAP server in Docker | NOT Microsoft Active Directory. AD has Kerberos, Group Policy, SYSVOL, trusts. OpenLDAP has none of these. Say "OpenLDAP-based directory" in interviews, never "Active Directory". |
| Keycloak | Open-source IdP in Docker | NOT Okta, Azure AD, or PingFederate. The OIDC/SAML protocols are the same; the admin UI and enterprise features differ. |
| step-ca | Open-source private CA | NOT a public CA (DigiCert, Let's Encrypt). Certs are trusted only within Groot. Fine for internal mTLS. |
| mTLS | Planned for Phase 4 | Not yet implemented. Do not claim it on resume until Phase 4 tests pass. |
| Audit log | Planned for Phase 9 | Not yet implemented. |

## What IS defensible right now (Phase 0)

- Go 1.22.5 installed from official tarball with checksum verified
- git worktree isolating IAM work from live main branch
- Backup: local + pendrive + git tag before any changes
- Ports planned and conflict-checked against live stack
- Architecture documented with honest RAM costs
- STRIDE model written before any code
