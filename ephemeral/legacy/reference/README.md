# Reference material

Upstream projects mounted as submodules so their source text can be read and
cited in place. Nothing here is built, tested, or imported by Gimbal.

## Read-only by policy

Reference submodules are mirrors. Do not commit inside one, do not push from
one, and do not carry a local edit forward. Changes to that material belong
upstream, in its own repository.

Git cannot enforce this, so the policy is backed by two settings:

- `.gitmodules` sets `ignore = dirty`, so scratch edits inside a reference tree
  never show up in Gimbal's status or diffs and cannot be staged by accident.
- Each clone points its push URL at a dead scheme, so `git push` from inside the
  submodule fails instead of reaching upstream.

The push URL lives in local config and does not survive a fresh clone. Restore
it after `git submodule update --init`:

```bash
git -C reference/diffusion-skills remote set-url --push origin no-push://read-only-mirror
```

## Updating a pin

Fetch upstream, move the checkout to the commit you want, and commit the new
pointer from the superproject:

```bash
git -C reference/diffusion-skills fetch --depth 1 origin main
git -C reference/diffusion-skills checkout FETCH_HEAD
git add reference/diffusion-skills
```

## Submodules

- `diffusion-skills` — github.com/diffusioninc/skills. The df-* skill bundles
  for chapters, sprint planning and execution, promise loops, semantic indexes,
  and the EasyLoop delivery workflows. Source material for Gimbal's built-in
  workflows.
