# Go analysis reference snapshot

Source: https://pkg.go.dev/golang.org/x/tools/go/analysis
Fetched: 2026-09-14
Purpose: feasibility notes for a Gimble analyzer and graph extractor.

The package defines a modular static analysis interface. An analyzer runs one
package at a time and can retain facts for use while analyzing importing
packages. The core `analysis.Pass` provides syntax files, type information,
`ResultOf` values from required analyzers, and diagnostic reporting.

Relevant page excerpts (page lines as fetched 2026-09-14):

> A static analysis is a function that inspects a package of Go code and
> reports a set of diagnostics ... and perhaps produces other results as well,
> such as suggested refactorings or other facts. (lines 185-190)

> The `Pass` describes a single unit of work ... The `Fset`, `Files`, `Pkg`,
> and `TypesInfo` fields provide the syntax trees and type information for a
> single package. (lines 235-250)

> The `ResultOf` field provides the results computed by analyzers required by
> this one ... `inspect` enables efficient syntax traversal and `buildssa`
> constructs SSA form. (lines 251-253)

> `Requires` establishes a horizontal dependency between analysis passes ...
> The graph over `Requires` edges must be acyclic. (lines 220-233)

> Facts can be associated with objects declared in the current package or with
> the package as a whole; facts are serialized with gob and can be imported
> from dependencies. An analyzer may export facts only for its current package
> or objects, while it may import facts from dependencies. (lines 271-301)

> `analysistest` checks expected diagnostics and facts with `// want ...`
> comments; `singlechecker` and `multichecker` provide standalone drivers.
> (lines 302-317)

Implications for Gimble: `inspect` plus `buildssa` is a valid in-package
foundation for finding calls, lexical closures, control flow, and SSA values.
Cross-package context and session summaries require exported analyzer facts;
facts cannot be emitted for arbitrary external objects. Diagnostics are
positioned and categorized, while severity policy belongs to the driver, so a
build-failing Gimble gate must make the analyzer's diagnostics fatal in its
driver or gate.
