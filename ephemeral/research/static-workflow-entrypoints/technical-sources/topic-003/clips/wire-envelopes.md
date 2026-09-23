# Wire Envelopes and Response Payloads for SvelteKit Forms

### 1. Request Binary Envelope (application/x-sveltekit-formdata)

SvelteKit's `serialize_binary_form` constructs a binary payload containing:
- 1 byte version (`0x00`)
- 4 bytes uint32 LE: header length $N$
- 2 bytes uint16 LE: file offset table length $M$
- $N$ bytes: `devalue.stringify([data, meta])`
- $M$ bytes: JSON string of file offsets relative to end of table (e.g. `"[0]"` or empty if no files)
- Raw file data concatenated (sorted smallest file first)

#### Example Golden Fixtures (from `internal/formdata` and `remote_form_test.go`):

1. **Standard Form Submission (`formGoldenSubmission`)**:
   - Base64: `AGAAAAAAAFtbMSw0XSx7ImZyb20iOjIsImJvZHkiOjN9LCJBZGEiLCJoZWxsbyB0aGVyZSIseyJyZW1vdGVfcmVmcmVzaGVzIjo1fSxbNl0sIndvcm9sYy9nZXRNZXNzYWdlcy8iXQ==`
   - Header payload represents:
     - `data`: `{from: "Ada", body: "hello there"}`
     - `meta`: `{remote_refreshes: ["worolc/getMessages/"]}`

2. **Validation-Only Request (`formGoldenValidateOnly`)**:
   - Base64: `AC4AAAAAAFtbMSwzXSx7ImZyb20iOjJ9LCIiLHsidmFsaWRhdGVfb25seSI6NH0sdHJ1ZV0=`
   - Header payload represents:
     - `data`: `{from: ""}`
     - `meta`: `{validate_only: true}`

---

### 2. Response JSON Envelopes (HTTP Status 200 OK)

All runtime remote responses return HTTP 200 with `Content-Type: application/json` and `Cache-Control: private, no-store`.

#### A. Success Result Envelope
```json
{
  "type": "result",
  "data": "[[1,2],{\"submission\":true,\"result\":3},{\"id\":4},\"m1\"]"
}
```
- In SvelteKit / SKGO, `data` is a devalue-serialized string.
- Devalue unflattening yields an object:
  ```json
  {
    "_": {
      "submission": true,
      "result": { "id": "m1" }
    },
    "r": true
  }
  ```
- If single-flight refreshes were executed, `r: true` is set, telling `form.svelte.js` to skip `refreshAll()`.

#### B. Field Validation Issues Envelope (Handler returns `*skgo.Invalid`)
```json
{
  "type": "result",
  "data": "[[1,2],{\"submission\":true,\"issues\":3},[4],{\"name\":5,\"path\":6,\"message\":7,\"server\":true},\"from\",[5],\"Tell us who you are\"]"
}
```
- Devalue unflattening yields:
  ```json
  {
    "_": {
      "submission": true,
      "issues": [
        {
          "name": "from",
          "path": ["from"],
          "message": "Tell us who you are",
          "server": true
        }
      ]
    }
  }
  ```
- Notice: `q`, `l`, and `r` are completely absent. Refreshes are suppressed to preserve page state and user inputs. Input is NOT echoed back.

#### C. Validation-Only Response (`meta.ValidateOnly == true`)
```json
{
  "type": "result",
  "data": "[[1],[]]"
}
```
- Devalue unflattening yields:
  ```json
  {
    "_": []
  }
  ```
- Empty array of issues. Handler execution is bypassed.

#### D. Server Failure Envelope (Runtime exception or unhandled error)
```json
{
  "type": "error",
  "error": {
    "status": 500,
    "message": "Internal Error"
  }
}
```
- Status is HTTP 200 OK!
- Envelope type is `"error"`, NOT `"result"`.
- Contains the error object with status code and message.
