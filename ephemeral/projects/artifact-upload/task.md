# Add provider-specific artifact uploading to Gimbal

Add a `gimbal upload-artifact` command that uploads one local file to a configured artifact-storage provider and returns a browser-accessible HTTPS URL. The intended use is publishing screenshots and other proof-of-completion artifacts. A successful invocation must write only the URL and one newline to stdout. Progress and labels must never appear there. Failures must leave stdout empty, explain the problem on stderr, and exit nonzero.

The command must support explicit `r2` and `s3` providers:

```shell
gimbal upload-artifact --provider r2 PATH_TO_FILE
gimbal upload-artifact --provider s3 PATH_TO_FILE
```

`GIMBAL_UPLOAD_PROVIDER` supplies the provider when the flag is absent, followed by the provider in the optional user-wide `~/.gimbal/config.json`; the flag takes precedence over both. Environment variables override corresponding file values. Do not infer the provider from available credentials. Do not retain the proposed generic `--api-key` and `--bucket-address` flags: those concepts cannot accurately configure either provider.

Follow the same broad pattern as Gimbal's harness adapters. Define a small, private uploader interface inside the CLI implementation whose single operation uploads a file and returns its public URL. Give R2 and AWS S3 independent implementations. They may duplicate small amounts of code; do not introduce a normalized cross-provider configuration, shared credential model, exported API, or generalized storage package. Factor common code only if the finished implementations reveal a genuinely useful common operation.

Each adapter constructor should read and validate its provider's configuration once, using an injected lookup function so tests do not depend on process-global state. The command composes that lookup from environment variables and the matching fields in `~/.gimbal/config.json`, with environment variables winning. The resulting uploader should retain that configuration and perform no further lookups during upload. A missing config file is equivalent to an empty one; an existing unreadable or malformed file is an error.

The R2 adapter reads `GIMBAL_R2_ACCOUNT_ID`, `GIMBAL_R2_ACCESS_KEY_ID`, `GIMBAL_R2_SECRET_ACCESS_KEY`, `GIMBAL_R2_BUCKET`, and `GIMBAL_R2_PUBLIC_BASE_URL`. It derives the R2 S3 endpoint from the account ID, uses region `auto`, and uploads with the provider's S3-compatible `PutObject` operation. It must not use Workers or require a separate HTTP service.

The S3 adapter reads `GIMBAL_S3_BUCKET` and `GIMBAL_S3_PUBLIC_BASE_URL`, while using the AWS SDK's normal region and credential chain, including `AWS_REGION`, environment credentials, and configured profiles. It uploads through AWS S3's ordinary `PutObject` operation. Neither adapter changes bucket policies, public-access settings, or lifecycle rules.

Store each artifact under an unguessable key containing an artifact prefix, the UTC upload date, a random identifier, and a safe form of the original basename. Stream the file, send it with `Content-Disposition: inline`, set its content type from the extension with explicit Markdown support and `application/octet-stream` as the fallback, and construct the result by joining the configured public base URL with the escaped object key. Do not use presigned URLs or per-object ACLs.

The deployment contract is that the selected bucket or artifact prefix is already publicly readable and has a lifecycle rule that removes artifacts after approximately 90 days. Anyone possessing a returned URL can read the artifact, so the command is not suitable for secrets or sensitive material. Gimbal does not configure, verify, or alter that retention policy and does not expose a TTL flag, deletion command, listing command, or multi-file upload in this work.

Add focused tests for provider selection, configuration precedence and parsing, provider construction, unreadable and non-regular files, content types and disposition, upload failures, URL escaping, and the exact stdout/stderr contract. Verify the built command's help as a caller. Run the relevant repository checks and demonstrate the real compiled CLI by uploading and retrieving identical bytes through the available live R2 account. AWS S3 is covered by focused adapter tests because this project has no live AWS account. Do not commit screenshots, logs, proof programs, or other run output; report what was personally observed.
