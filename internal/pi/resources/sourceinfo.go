package resources

// SourceScope identifies where a resource came from.
type SourceScope string

// Source scopes.
const (
	ScopeUser      SourceScope = "user"
	ScopeProject   SourceScope = "project"
	ScopeTemporary SourceScope = "temporary"
)

// SourceOrigin distinguishes a resource declared by a package from one
// declared directly.
type SourceOrigin string

// Source origins.
const (
	OriginPackage  SourceOrigin = "package"
	OriginTopLevel SourceOrigin = "top-level"
)

// SourceInfo describes where a resource was loaded from. It mirrors pi's
// SourceInfo.
type SourceInfo struct {
	Path    string
	Source  string
	Scope   SourceScope
	Origin  SourceOrigin
	BaseDir string
}

// CreateSourceInfo builds a SourceInfo from package metadata.
func CreateSourceInfo(path string, metadata PathMetadata) SourceInfo {
	return SourceInfo{
		Path:    path,
		Source:  metadata.Source,
		Scope:   metadata.Scope,
		Origin:  metadata.Origin,
		BaseDir: metadata.BaseDir,
	}
}

// SyntheticSourceInfoOptions are the optional fields of
// CreateSyntheticSourceInfo.
type SyntheticSourceInfoOptions struct {
	Source  string
	Scope   SourceScope
	Origin  SourceOrigin
	BaseDir string
}

// CreateSyntheticSourceInfo builds a SourceInfo for a path with no package
// metadata. Missing scope and origin default to temporary and top-level.
func CreateSyntheticSourceInfo(path string, options SyntheticSourceInfoOptions) SourceInfo {
	scope := options.Scope
	if scope == "" {
		scope = ScopeTemporary
	}
	origin := options.Origin
	if origin == "" {
		origin = OriginTopLevel
	}
	return SourceInfo{
		Path:    path,
		Source:  options.Source,
		Scope:   scope,
		Origin:  origin,
		BaseDir: options.BaseDir,
	}
}
