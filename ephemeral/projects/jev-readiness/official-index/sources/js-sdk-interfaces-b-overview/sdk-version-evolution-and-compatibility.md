# JavaScript SDK overview, version evolution, and compatibility

## Purpose

This leaf records the JavaScript SDK’s baseline runtime/package contract, public API surface, and the only documented version transition. It is the compatibility checkpoint for examples or integrations written against the initial release.

## Baseline SDK contract

- The package is the JavaScript/TypeScript SDK for TypeSafe AI and requires Node.js 20 or newer in the quickstart. Installation is `npm install @typesafe-ai/sdk`. [javascript](https://docs.typesafe.ai/sdk/javascript.md)
- The quickstart creates `TypeSafeClient`, sends structured state plus a typed `choice`, and reads `response.answers.category.choice`. [javascript](https://docs.typesafe.ai/sdk/javascript.md)
- Answer types are inferred from questions. The package ships ESM, CommonJS, and TypeScript declarations. [javascript](https://docs.typesafe.ai/sdk/javascript.md)
- The docs link directly to client and type sources pinned at repository tag `v0.6.0`, anchoring the generated reference to that release. [javascript](https://docs.typesafe.ai/sdk/javascript.md)

## Public surface inventory

- The API index lists typed transport/error classes including connection, timeout, user abort, authentication, permission, rate-limit, validation, and server errors, plus `APIPromise` and `TypeSafeClient`. [api](https://docs.typesafe.ai/sdk/javascript/api.md)
- It lists all question/response, model, request/retry, system-one, config, usage, and raw-response interfaces. [api](https://docs.typesafe.ai/sdk/javascript/api.md)
- Type aliases include criteria, entry/JSON types, log level, question/result mappings, and score helpers. Variables include environment mapping, log-level constants, and SDK version. Public constructors are `choice`, `noul`, and `score`. [api](https://docs.typesafe.ai/sdk/javascript/api.md)

## Version history and breaking change

- v0.5.7, dated 2026-09-11, is the initial public JavaScript/TypeScript SDK release. [changelog](https://docs.typesafe.ai/sdk/javascript/changelog.md)
- v0.6.0, dated 2026-09-15, made a breaking change: `Score.criteria` now accepts an ordered sequence instead of a dictionary keyed by integers. [changelog](https://docs.typesafe.ai/sdk/javascript/changelog.md)

## Citation bookmarks

- Node/package/quickstart: [javascript](https://docs.typesafe.ai/sdk/javascript.md)
- Source pin: [javascript](https://docs.typesafe.ai/sdk/javascript.md)
- API inventory: [api](https://docs.typesafe.ai/sdk/javascript/api.md)
- Complete changelog: [changelog](https://docs.typesafe.ai/sdk/javascript/changelog.md)

## Themes

- **Strong TypeScript ergonomics:** authored question literals drive answer types.
- **Broad module compatibility:** ESM and CommonJS are both first-class package outputs.
- **Young, evolving API:** the first breaking change arrived four days after initial public release.
- **Ordered Score criteria are semantic:** sequence position is now the level identity; object-key enumeration should no longer define rubric order.

## Gotchas and gaps

- Code written for v0.5.7 with `{0: ..., 1: ...}` Score criteria needs migration to an ordered sequence before v0.6.0 compatibility can be assumed.
- The changelog gives no migration example, deprecation period, compatibility shim, or statement about wire-format compatibility.
- Only two versions are documented here; there is no release history for later fixes, supported Node minor versions, browser/runtime matrix, or semantic-version policy.
- Generated API docs appear pinned to v0.6.0, so they should not be treated as automatically describing any newer installed package.
- The API inventory lists error types but these assigned sources do not document their fields, inheritance, status mapping, or retry implications.
- Nothing in the overview or API inventory declares image/vision input support. `EntryType` is described elsewhere as text/JSON object/array/null, so multimodal support cannot be inferred from this JavaScript surface.

## Task recipes

1. **Audit an integration:** record installed package version, Node version, module mode, Score criteria shape, and generated-doc source tag before attributing runtime behavior to the docs.
2. **Migrate v0.5.7 Score code:** replace integer-keyed criteria objects with explicit ordered arrays/sequences; add tests that assert level order, legend order, and expected-score interpretation.
3. **Pin and probe:** pin an exact SDK version for reproducible Gimbal experiments; run compile and mock-transport contract tests before upgrading.
4. **Separate compatibility axes:** test TypeScript source compatibility, emitted module loading (ESM/CJS), HTTP payload shape, and behavioral/model drift independently.
