package workspace

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/tylergannon/gimbal/internal/app"
	"github.com/tylergannon/gimbal/internal/host"
	"github.com/tylergannon/gimbal/internal/projectfiles"
	"github.com/tylergannon/gimbal/internal/skgo/params"
	"github.com/tylergannon/skgo"
	"github.com/yuin/goldmark"
)

func project(ctx context.Context, id string) (*host.Project, error) {
	o := host.OwnerFrom(ctx)
	if o != nil {
		for _, p := range o.Projects() {
			if p.ID() == id {
				return p, nil
			}
		}
	}
	return nil, skgo.Errorf(404, "Project not found")
}

type NewProject struct {
	Path string `json:"path"`
}

func createProject(ctx context.Context, event app.RequestEvent[params.Params], r NewProject) (host.ProjectChoice, error) {
	o := host.OwnerFrom(ctx)
	if o == nil {
		return host.ProjectChoice{}, skgo.Errorf(503, "Server unavailable")
	}
	path := strings.TrimSpace(r.Path)
	if !filepath.IsAbs(path) {
		return host.ProjectChoice{}, skgo.Invalidf("path", "Use an absolute project directory")
	}
	if e := os.MkdirAll(path, 0755); e != nil {
		return host.ProjectChoice{}, e
	}
	p, e := o.AdmitProject(path)
	if e != nil {
		return host.ProjectChoice{}, e
	}
	return host.ProjectChoice{ID: p.ID(), Path: p.Path()}, nil
}

type Upload struct {
	Project string    `json:"project"`
	File    skgo.File `json:"file"`
}

func uploadFile(ctx context.Context, event app.RequestEvent[params.Params], r Upload) (projectfiles.Document, error) {
	p, e := project(ctx, r.Project)
	if e != nil {
		return projectfiles.Document{}, e
	}
	doc, err := projectfiles.Upload(ctx, p.Dir(), p.ID(), r.File.Name, r.File.Data)
	if err != nil {
		return doc, err
	}
	return doc, skgo.RefreshRequested(ctx, projectFiles, 1)
}
func projectFiles(ctx context.Context, id string) ([]projectfiles.Document, error) {
	p, e := project(ctx, id)
	if e != nil {
		return nil, e
	}
	return projectfiles.List(p.Dir())
}

var _ = skgo.Form(createProject)
var _ = skgo.Form(uploadFile)
var _ = skgo.Query(projectFiles)

type FileRequest struct {
	Project string `json:"project"`
	ID      string `json:"id"`
}
type FileView struct {
	HTML string                `json:"html"`
	File projectfiles.Document `json:"file"`
	URL  string                `json:"url"`
	Text string                `json:"text"`
}

func viewFile(ctx context.Context, r FileRequest) (FileView, error) {
	p, e := project(ctx, r.Project)
	if e != nil {
		return FileView{}, e
	}
	f, e := projectfiles.Get(p.Dir(), r.ID)
	if e != nil {
		return FileView{}, e
	}
	url, e := projectfiles.ViewURL(ctx, f)
	if e != nil {
		return FileView{}, e
	}
	out := FileView{File: f, URL: url}
	if strings.HasPrefix(f.MediaType, "text/") {
		body, _, _, e := projectfiles.Body(ctx, f, "")
		if e != nil {
			return FileView{}, e
		}
		defer func() { _ = body.Close() }()
		data, e := io.ReadAll(body)
		if e != nil {
			return FileView{}, e
		}
		out.Text = string(data)
		if strings.HasSuffix(strings.ToLower(f.Name), ".md") || strings.HasPrefix(f.MediaType, "text/markdown") {
			var html bytes.Buffer
			if e := goldmark.Convert(data, &html); e != nil {
				return FileView{}, e
			}
			out.HTML = html.String()
		}
	}
	return out, nil
}

var _ = skgo.Query(viewFile)
