package taskdaemon

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"

	"github.com/tylergannon/gimbal/backend"
)

// PublicHost is an explicitly configured address reachable by the service's consumer.
func (d *Daemon) service(v backend.Service) (backend.ServiceAddress, error) {
	if v.Port < 1 || v.Port > 65535 {
		return backend.ServiceAddress{}, fmt.Errorf("invalid service TCP port")
	}
	if d.PublicHost == "" {
		return backend.ServiceAddress{}, fmt.Errorf("service consumer topology unavailable: daemon requires public host")
	}
	probe, err := net.Listen("tcp", net.JoinHostPort("", strconv.Itoa(v.Port)))
	if err != nil {
		return backend.ServiceAddress{}, fmt.Errorf("service port unavailable: %w", err)
	}
	_ = probe.Close()
	v.Service = true
	if err := d.process(v.Process); err != nil {
		return backend.ServiceAddress{}, err
	}
	return backend.ServiceAddress{ID: v.Task, URL: "http://" + net.JoinHostPort(d.PublicHost, strconv.Itoa(v.Port))}, nil
}
func (d *Daemon) serviceHandler(m *http.ServeMux) {
	m.HandleFunc("POST /tasks/service", func(w http.ResponseWriter, r *http.Request) {
		v, ok := decode[backend.Service](w, r)
		if !ok {
			return
		}
		address, err := d.service(v)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
		_ = json.NewEncoder(w).Encode(address)
	})
}
func (d *Daemon) workdir(requested string) string {
	if d.Workdir != "" {
		return d.Workdir
	}
	return requested
}
