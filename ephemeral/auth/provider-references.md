# Provider and session references

These are factual notes collected from the official pages below on 2026-10-12,
so a reviewer has the relevant external material locally. Design choices are
in design.md, not in these notes.

## GitHub

Source: https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/authorizing-oauth-apps

- The authorization-code web flow redirects to GitHub, returns a code to the
  application, and exchanges it at the token endpoint.
- GitHub recommends unpredictable state, a redirect URI, and PKCE using S256.
  State returned by the callback must match the initiating flow; otherwise the
  application should abort. Token exchange accepts the original code verifier.
- The application calls GET https://api.github.com/user with the access token
  and must revalidate identity after every sign-in.

Source: https://www.rfc-editor.org/rfc/rfc6749#section-4.1.2

- Authorization codes must not be used more than once. The authorization
  server must deny an attempt to reuse one, and should revoke tokens previously
  issued from that code when possible. This is a provider obligation; a
  stateless client flow cookie does not itself enforce one-use flow state.

Source: https://docs.github.com/en/rest/users/users#get-the-authenticated-user

- The authenticated-user response includes a numeric id and a login name.

Source: https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/scopes-for-oauth-apps

- No requested scope grants read-only access to public information, including
  user profile information. Repository, organization, and private-email
  permissions are separate scopes.

## Google (future provider)

Source: https://developers.google.com/identity/openid-connect/openid-connect

- Google's login protocol is OpenID Connect. The authorization request can
  carry a nonce, and validation must check it against the ID token.
- ID tokens must be validated for signature, issuer, audience, and expiry.
- The sub identifies the Google account and is never reused. Email is mutable
  and must not be used as the account's primary identifier.

## Sessions

Source: https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html

- Session identifiers should be generated with a cryptographically secure
  random generator; cookies should use Secure and HttpOnly with an appropriate
  SameSite policy.
- Session expiry and invalidation must be enforced on the server. Removing a
  browser cookie alone does not invalidate its session server-side.
- Avoid session identifiers in URLs and logs. Transport protection and
  protection against cross-site state-changing requests are separate needs.
