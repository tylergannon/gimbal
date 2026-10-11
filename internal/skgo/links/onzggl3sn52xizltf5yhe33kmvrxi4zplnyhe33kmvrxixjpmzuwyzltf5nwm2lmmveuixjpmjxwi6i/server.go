package filebody

import (
	"io"
	"mime"
	"net/http"
	"strconv"

	"github.com/tylergannon/gimbal/internal/host"
	"github.com/tylergannon/gimbal/internal/projectfiles"
	"github.com/tylergannon/skgo"
)

func serve(w http.ResponseWriter, r *http.Request) {
	event := skgo.EventFrom(r.Context())
	f, e := projectfiles.Get(host.ProjectDir(r.Context()), event.Param("fileID"))
	if e != nil {
		http.Error(w, e.Error(), 404)
		return
	}
	body, length, contentRange, e := projectfiles.Body(r.Context(), f, r.Header.Get("Range"))
	if e != nil {
		http.Error(w, e.Error(), http.StatusBadGateway)
		return
	}
	defer func() { _ = body.Close() }()
	w.Header().Set("Content-Type", f.MediaType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": f.Name}))
	w.Header().Set("Content-Length", strconv.FormatInt(length, 10))
	w.Header().Set("Accept-Ranges", "bytes")
	if contentRange != "" {
		w.Header().Set("Content-Range", contentRange)
		w.WriteHeader(206)
	}
	_, _ = io.Copy(w, body)
}

var _ = skgo.GET(serve)
