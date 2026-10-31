# ADR 0005: Email scope and staff onboarding

## Status

Accepted.

## Context

The brief explicitly requires one email: "a reset link should arrive at their
registered email". It says nothing about invitations or email verification.

## Decision

- Password reset is emailed. Tokens are 256-bit, stored hashed, single-use, and
  expire after 30 minutes.
- Email verification is optional (soft): registration logs the user in
  immediately, the API exposes `email_verified`, and the UI shows a banner. The
  verification token expires after 48 hours.
- Staff onboarding does not use email. The admin creates the staff member, the
  API generates a 16-character password satisfying the policy, returns it once,
  and sets `must_change_password`. The staff member changes it at first login.
- Development uses Mailpit. Production uses SMTP. If `SMTP_HOST` is empty the
  mailer logs the message instead of sending, so local development never blocks.

## Consequences

- The only mandatory email dependency is password reset.
- No invite-token flow and no "pending" user state.
- The temporary password is visible in the admin's browser once; if lost, the
  admin must reset it (a future endpoint).
