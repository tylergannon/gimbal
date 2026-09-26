// Package files holds the shared file, path, truncation, and image helpers
// used by pi's built-in tools and their callers.
//
// It is a hand port of the following upstream modules (pin d6af72e1):
//
//   - core/tools/path-utils.ts
//   - core/tools/truncate.ts
//   - core/tools/file-mutation-queue.ts
//   - utils/paths.ts
//   - utils/image-process.ts
//   - utils/tool-result-images.ts
//
// The image pipeline also carries the behavior those modules depend on from
// utils/image-convert.ts, utils/image-resize.ts, utils/image-resize-core.ts and
// utils/exif-orientation.ts.
package files
