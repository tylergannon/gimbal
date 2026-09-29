# Courier capacity reference
Revision: 2026-02-10. Applies to version 2.

A v2 request may contain at most 80 messages. Each message may contain at most 12 KiB of body data. The aggregate body data in one request may not exceed 256 KiB. These constraints apply simultaneously; the aggregate cap is not a per-message cap. Metadata is counted separately and is not included in either body-data number.

The service rejects the whole request when any body limit is exceeded. A rejected request does not enqueue a valid prefix. The body encoding is UTF-8; callers should measure bytes after encoding. This document does not specify a monetary price, requests per second, or an attachment limit.

This reference supersedes the v1 body-size section for v2 clients only. It does not change legacy clients. The product name and version must accompany a quoted capacity.
