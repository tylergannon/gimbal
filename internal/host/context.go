package host

import "context"

type projectDirKey struct{}
type projectsKey struct{}
type ownerKey struct{}

func WithOwner(ctx context.Context, owner *Owner) context.Context {
	return context.WithValue(ctx, ownerKey{}, owner)
}

func OwnerFrom(ctx context.Context) *Owner {
	owner, _ := ctx.Value(ownerKey{}).(*Owner)
	return owner
}

type ProjectChoice struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}

func WithProjects(ctx context.Context, choices []ProjectChoice) context.Context {
	return context.WithValue(ctx, projectsKey{}, choices)
}

func Projects(ctx context.Context) []ProjectChoice {
	choices, _ := ctx.Value(projectsKey{}).([]ProjectChoice)
	return choices
}

func WithProjectDir(ctx context.Context, dir string) context.Context {
	return context.WithValue(ctx, projectDirKey{}, dir)
}

func ProjectDir(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	dir, _ := ctx.Value(projectDirKey{}).(string)
	return dir
}
