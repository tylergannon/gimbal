# SvelteKit 3 Remote Form Client Runtime and Field Proxy

- **Origin**: `/Users/tyler/src/skgo/ephemeral/inspiration/reference/kit@3.0.0-next.25`
- **Files**:
  - `src/runtime/client/remote-functions/form.svelte.js`
  - `src/runtime/form-utils.js`
  - `src/runtime/client/remote-functions/shared.svelte.js`
- **Reference Version**: `@sveltejs/kit@3.0.0-next.25`
- **Retrieval Date**: 2026-09-23

---

## 1. Remote Form Submission and Response Handling

From `src/runtime/client/remote-functions/form.svelte.js` (lines 246–286):

```javascript
const { blob } = serialize_binary_form(convert(form_data), {
    remote_refreshes: Array.from(refreshes ?? [])
});

const response = await remote_request(
    `${base}/${app_dir}/remote/${action_id_without_key}`,
    {
        method: 'POST',
        headers: {
            'Content-Type': BINARY_FORM_CONTENT_TYPE,
            // Forms cannot be called during rendering, so it's safe to use location here
            'x-sveltekit-pathname': location.pathname,
            'x-sveltekit-search': location.search
        },
        body: blob
    }
);

({ issues: raw_issues = [], result } = response._ ?? {});

// if the developer took control of updates via `.updates(...)` (even with
// no arguments), or the server performed explicit refreshes, don't invalidateAll
const should_refresh = refreshes === null && !response.r;

if (response.redirect) {
    // Use internal version to allow redirects to external URLs
    await _goto(response.redirect, {
        refreshAll: should_refresh
    });
    return true;
}

const succeeded = raw_issues.length === 0;

if (succeeded) {
    if (should_refresh) {
        await refreshAll();
    }
} else {
    if (DEV) {
        warn_on_missing_issue_reads();
    }
}

return succeeded;
```

---

## 2. Field Proxy Accessor and Reactive Binding

From `src/runtime/client/remote-functions/form.svelte.js` (lines 640–675):

```javascript
fields: {
    get: () =>
        create_field_proxy({
            form_id: action_id_without_key,
            get: () => input,
            set: (path, value) => {
                if (path.length === 0) {
                    input = value;
                } else if (value !== deep_get(input, path)) {
                    deep_set(input, path.map(String), value);

                    const key = build_path_string(path);

                    if (element) {
                        touched[key] = true;
                        dirty[key] = true;
                        can_validate[key] = true;
                    }
                }
            },
            get_issues: (path, all) => {
                if (DEV && unread_issues !== null && path !== undefined) {
                    unread_issues = unread_issues.filter((issue) => {
                        return (
                            (all ? issue.path.slice(0, path.length) : issue.path).join('.') !==
                            path.join('.')
                        );
                    });
                }

                return issues;
            },
            get_touched: () => touched,
            get_dirty: () => dirty
        })
},
result: {
    get: () => result
},
pending: {
    get: () => pending_count
},
submitted: {
    get: () => submitted
},
```

---

## 3. Field `.as(...)` Properties and `aria-invalid` Generation

From `src/runtime/form-utils.js` (lines 752–807, 897–913):

```javascript
if (prop === 'as') {
    /**
     * @param {string} type
     * @param {unknown} [input_value]
     */
    const as_func = (type, input_value) => {
        const is_array =
            type === 'file multiple' ||
            type === 'select multiple' ||
            (type === 'checkbox' && typeof input_value === 'string');

        const type_prefix = get_type_prefix(type, is_array, input_value);

        // Base properties for all input types
        /** @type {Record<string, any>} */
        const base_props = {
            name: type_prefix + key + (is_array ? '[]' : '') + '/' + context.form_id,
            get 'aria-invalid'() {
                const issues = context.get_issues();
                return key in issues ? 'true' : undefined;
            }
        };

        // Add type attribute only for non-text inputs and non-select elements
        if (type !== 'text' && type !== 'select' && type !== 'select multiple') {
            base_props.type = type === 'file multiple' ? 'file' : type;
        }
        ...
        // Handle all other input types (text, number, etc.)
        return Object.defineProperties(base_props, {
            defaultValue: {
                enumerable: true,
                get() {
                    return input_value;
                }
            },
            value: {
                enumerable: true,
                get() {
                    const value = get_value() ?? input_value;
                    return value != null ? String(value) : '';
                }
            }
        });
    };

    return create_field_proxy(context, as_func, next);
}
```

---

## 4. Server Error Envelopes in `remote_request`

From `src/runtime/client/remote-functions/shared.svelte.js` (lines 120–137):

```javascript
if (!response.ok) {
    const result = await response.json().catch(() => undefined);

    throw result?.type === 'error'
        ? new HandledHttpError({ status, ...result.error })
        : new HttpError({ status, message: response.statusText });
}

const result = /** @type {RemoteFunctionResponse} */ (await response.json());

if (result.type === 'error') {
    throw new HandledHttpError(result.error);
}

const data = /** @type {RemoteFunctionData} */ (
    result.data ? devalue.parse(result.data, app.decoders) : {}
);
```
