package web

import (
	"context"
	"io/fs"
	"net/http"
	"os"
	"sync"

	"github.com/tylergannon/skgo"
)

// Handler reconstruction must retain the catalog, admitted projects and control
// socket. A structural rebuild starts a new process and restores durable state.
var developmentInstance = sync.OnceValues(func() (*Instance, error) {
	dir := os.Getenv("GIMBAL_INSTANCE_DIR")
	if dir == "" {
		dir = ".gimbal"
	}
	return NewInstance(context.Background(), dir, nil, WithNoWeb())
})

// NewDevelopmentHandler supplies Gimbal's real application context to Kit's Go
// callbacks. The development coordinator owns the public listener.
func NewDevelopmentHandler(dist fs.FS, origin string, development *skgo.NativeDevelopment) (http.Handler, error) {
	instance, err := developmentInstance()
	if err != nil {
		return nil, err
	}
	handler, _, err := newHandler(dist, origin, development, handlerSettings{project: instance.projectRequest})
	return handler, err
}
