package resources

// ResourceDiagnostic kinds, mirroring pi's ResourceDiagnostic type field.
const (
	DiagnosticWarning   = "warning"
	DiagnosticError     = "error"
	DiagnosticCollision = "collision"
)

// ResourceCollision describes a name collision between two resources of the
// same type. The first resource loaded wins.
type ResourceCollision struct {
	ResourceType string
	Name         string
	WinnerPath   string
	LoserPath    string
}

// ResourceDiagnostic is a load warning, error or name collision with the
// offending path.
type ResourceDiagnostic struct {
	Type      string
	Message   string
	Path      string
	Collision *ResourceCollision
}
