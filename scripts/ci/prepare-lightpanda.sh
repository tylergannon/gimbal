#!/usr/bin/env bash
# Bob's browser build. Cache by upstream source and our complete patch.
set -euo pipefail

if [[ $(uname -sm) != 'Linux x86_64' ]]; then
  echo 'This CI build targets Bob (Linux x86_64).' >&2
  exit 1
fi

patch_file="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lightpanda.patch"
revision=9c97385c277a29138ccd69f66e3ed4bbd03a869e
patch_hash=$(sha256sum "$patch_file" | cut -d' ' -f1)
cache="${XDG_CACHE_HOME:-$HOME/.cache}/gimbal/lightpanda"
build="$cache/$revision-$patch_hash"
tools="$cache/tools"
mkdir -p "$build" "$tools"
exec 9>"$cache/build.lock"
flock 9

if [[ ! -x "$build/source/zig-out/bin/lightpanda" ]]; then
  zig_dir="$tools/zig-x86_64-linux-0.17.0"
  if [[ ! -x "$zig_dir/zig" ]]; then
    curl --fail --location --retry 3 \
      https://ziglang.org/download/0.17.0/zig-x86_64-linux-0.17.0.tar.xz \
      --output "$tools/zig.tar.xz" >&2
    echo "1cbe9df9f27e6b78d14ccbca43b6703a404ef79ef1c463de901d7f088d4e2026  $tools/zig.tar.xz" | sha256sum --check >&2
    tar -xJf "$tools/zig.tar.xz" -C "$tools"
  fi
  export CARGO_HOME="$tools/cargo" RUSTUP_HOME="$tools/rustup"
  export PATH="$zig_dir:$CARGO_HOME/bin:$PATH"
  if [[ ! -x "$CARGO_HOME/bin/rustup" ]]; then
    curl --fail --location --retry 3 https://sh.rustup.rs --output "$tools/rustup-init.sh" >&2
    sh "$tools/rustup-init.sh" -y --no-modify-path --profile minimal --default-toolchain none >&2
  fi
  rustup toolchain install 1.94.0 --profile minimal >&2
  export RUSTUP_TOOLCHAIN=1.94.0

  if [[ ! -d "$build/source/.git" ]]; then
    git init --quiet "$build/source"
    git -C "$build/source" fetch --depth 1 https://github.com/lightpanda-io/browser.git "$revision" >&2
    git -C "$build/source" checkout --quiet --detach FETCH_HEAD
  fi
  cd "$build/source"
  if ! git apply --unidiff-zero --reverse --check "$patch_file" 2>/dev/null; then
    git apply --unidiff-zero "$patch_file"
  fi
  make download-v8 >&2
  # Match the Ubuntu 24.04 runner's libc even when preparing on Bob's Arch host.
  zig build -Doptimize=fast -Dtarget=x86_64-linux-gnu.2.39 >&2
fi

binary="$build/source/zig-out/bin/lightpanda"
"$binary" version >&2
if [[ -n ${GITHUB_ENV:-} ]]; then
  echo "LIGHTPANDA_BINARY=$binary" >> "$GITHUB_ENV"
fi
printf '%s\n' "$binary"
