# Authoritative request: optional authentication

Tyler requested updating the root checkout from origin/main, then designing a
way to lock down Gimbal that can be turned on. Main was fast-forwarded to
`c05eb1aa` before this design work.

The user requirements, in conversation order:

- "SUPER SIMPLE. no organizations. just users."
- "another plugin-based thing, you have to provide a small interface for doing
  that."
- "maybe actually we should only support oauth through google or github.
  github is easiest. we could just start with github support."
- "we just need to design an interface that must be provided via a plugin and
  selected at runtime."
- "user should be an interface so that customer code can define
  implementations, right?"
- "assume that our application will define the endpoints for oauth, so the
  plugin should be providing things like ChangePasswoed(), CreateUser(), and
  basically that gamut across the needed operations in order to support oauth
  with github/google. actually come to think of it there's no ChangePassword()
  in that situation right?"
- On durable local application users and a user-storage contract: "whatever's
  going to be durable. one extra api impl isn't a big deal"
- "let's get a design and consensus with Claude Opus"

This task produces a design and independent consensus review. It does not
implement authentication. Repository instructions remain authoritative.
