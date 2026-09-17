package gimble

// Env is the Gimble-owned environment passed to every generated workflow.
// WorkDir is the absolute initial working directory selected for the run.
type Env struct {
	WorkDir string
}
