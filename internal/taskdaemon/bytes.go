package taskdaemon

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/tylergannon/gimbal/backend"
)

func (d *Daemon) bytes(w http.ResponseWriter, r *http.Request) {
	v, ok := decode[backend.Artifact](w, r)
	if !ok {
		return
	}
	d.mu.Lock()
	t := d.tasks[key(v.Run, v.Task)]
	d.mu.Unlock()
	if t == nil {
		http.Error(w, "task not found", 404)
		return
	}
	t.mu.Lock()
	path := t.captures[v.Name]
	t.mu.Unlock()
	if path == "" {
		http.Error(w, "artifact not found", 404)
		return
	}
	file, e := os.Open(path)
	if e != nil {
		http.Error(w, e.Error(), 404)
		return
	}
	defer func() { _ = file.Close() }()
	_, _ = io.Copy(w, file)
}
func (d *Daemon) bytesHandlers(m *http.ServeMux) {
	m.HandleFunc("POST /tasks/bytes", d.bytes)

	m.HandleFunc("POST /tasks/write-bytes", func(w http.ResponseWriter, r *http.Request) {
		var v struct {
			backend.Artifact
			Bytes []byte
		}
		if e := json.NewDecoder(r.Body).Decode(&v); e != nil {
			http.Error(w, e.Error(), 400)
			return
		}
		reply(w, "", false, d.writeBytes(v.Artifact, bytes.NewReader(v.Bytes)))
	})
}

func (d *Daemon) writeBytes(v backend.Artifact, r io.Reader) error {
	if v.Run == "" || v.Task == "" || v.Name == "" || filepath.Base(v.Name) != v.Name {
		return errors.New("invalid task file identity")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.admit(v.Owner); err != nil {
		return err
	}
	file, err := os.CreateTemp("", "gimbal-task-input-*"+filepath.Ext(v.Name))
	if err != nil {
		return err
	}
	_, err = io.Copy(file, r)
	err = errors.Join(err, file.Close())
	if err != nil {
		_ = os.Remove(file.Name())
		return err
	}
	if old := d.files[v]; old != "" {
		_ = os.Remove(old)
	}
	d.files[v] = file.Name()
	return nil
}
