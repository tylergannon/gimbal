package hooks

import "context"

type projectDirKey struct{}

// WithProjectDir keeps the runtime's project directory on the request context
// so server loads can discover the runs that belong to this application.
func WithProjectDir(ctx context.Context, dir string) context.Context {
	return context.WithValue(ctx, projectDirKey{}, dir)
}

// ProjectDir returns the runtime's project directory from ctx.
func ProjectDir(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	dir, _ := ctx.Value(projectDirKey{}).(string)
	return dir
}
