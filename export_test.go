package gimble

import "context"

// CancelTurn kills the running turn id in the ctx's run, for the live
// attestation in package gimble_test, which imports the codex adapter and
// so cannot be an internal test. The web runtime exports the real thing.
func CancelTurn(ctx context.Context, id string, cause error) error {
	s, err := current(ctx)
	if err != nil {
		return err
	}
	return s.run.cancelTurn(id, cause)
}
