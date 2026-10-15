package environments

import (
	"os"
	"testing"
)

func TestResolveDirectoryUsesValidatedMachineWithoutSavingOrStarting(t *testing.T) {
	service := testService(t)
	if err := service.RegisterProject("p", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	raw := `{"version":1,"machines":[{"id":"queue","name":"Queue","kind":"route","route":{"Name":"future-backend","Configuration":{"endpoint":"http://127.0.0.1:1","opaque":{"retain":[1,2]}}},"future":{"keep":true}},{"id":5}],"environments":[]}`
	document, err := service.SaveConfig(t.Context(), ConfigEdit{Text: raw})
	if err != nil {
		t.Fatal(err)
	}
	target, err := service.ResolveDirectory(t.Context(), "p", "queue", "/new/directory")
	if err != nil {
		t.Fatal(err)
	}
	if target.Environment.ID != "" || target.Directory != "/new/directory" || target.Machine.ID != "queue" || target.Route.Name != "future-backend" {
		t.Fatalf("wrong prospective target: %+v", target)
	}
	if string(target.Machine.Extra["future"]) != `{"keep":true}` || string(target.Route.Configuration) != `{"endpoint":"http://127.0.0.1:1","opaque":{"retain":[1,2]}}` {
		t.Fatalf("opaque machine fields were changed: %+v", target)
	}
	after, err := service.Config(t.Context())
	if err != nil || after.Text != document.Text || after.Revision != document.Revision {
		t.Fatal("resolving a directory changed configuration", err)
	}
	service.mu.Lock()
	connections := len(service.connections)
	service.mu.Unlock()
	if connections != 0 {
		t.Fatal("resolving a directory opened a backend connection")
	}
	if _, err := service.ResolveDirectory(t.Context(), "unregistered", "queue", "/new"); err == nil {
		t.Fatal("unregistered project accepted")
	}
	if _, err := service.ResolveDirectory(t.Context(), "p", "missing", "/new"); err == nil {
		t.Fatal("invalid or absent machine accepted")
	}
	if err := os.WriteFile(service.path, []byte("{broken"), 0600); err != nil {
		t.Fatal(err)
	}
	retained, err := service.ResolveDirectory(t.Context(), "p", "queue", "/new/directory")
	if err != nil || retained.Machine.ID != "queue" {
		t.Fatal("malformed hand edit discarded last-known-good machine", err)
	}
}
