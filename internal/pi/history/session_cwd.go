package history

import "os"

// SessionCwdIssue describes a stored session whose working directory no longer
// exists.
type SessionCwdIssue struct {
	SessionFile string
	SessionCwd  string
	FallbackCwd string
}

// SessionCwdSource is the part of a session the missing-cwd check reads.
type SessionCwdSource interface {
	GetCwd() string
	GetSessionFile() string
}

// GetMissingSessionCwdIssue reports a stored session cwd that does not exist,
// or nil.
func GetMissingSessionCwdIssue(source SessionCwdSource, fallbackCwd string) *SessionCwdIssue {
	sessionFile := source.GetSessionFile()
	if sessionFile == "" {
		return nil
	}
	sessionCwd := source.GetCwd()
	if sessionCwd == "" {
		return nil
	}
	if _, err := os.Stat(sessionCwd); err == nil {
		return nil
	}
	return &SessionCwdIssue{SessionFile: sessionFile, SessionCwd: sessionCwd, FallbackCwd: fallbackCwd}
}

// FormatMissingSessionCwdError renders the controlled error text.
func FormatMissingSessionCwdError(issue SessionCwdIssue) string {
	sessionFile := ""
	if issue.SessionFile != "" {
		sessionFile = "\nSession file: " + issue.SessionFile
	}
	return "Stored session working directory does not exist: " + issue.SessionCwd + sessionFile +
		"\nCurrent working directory: " + issue.FallbackCwd
}

// FormatMissingSessionCwdPrompt renders the prompt shown before continuing in
// the fallback cwd.
func FormatMissingSessionCwdPrompt(issue SessionCwdIssue) string {
	return "cwd from session file does not exist\n" + issue.SessionCwd +
		"\n\ncontinue in current cwd\n" + issue.FallbackCwd
}

// MissingSessionCwdError is returned when a stored cwd cannot be used.
type MissingSessionCwdError struct {
	Issue SessionCwdIssue
}

// Error implements error.
func (e *MissingSessionCwdError) Error() string {
	return FormatMissingSessionCwdError(e.Issue)
}

// AssertSessionCwdExists returns a MissingSessionCwdError when the session's
// stored cwd is missing.
func AssertSessionCwdExists(source SessionCwdSource, fallbackCwd string) error {
	issue := GetMissingSessionCwdIssue(source, fallbackCwd)
	if issue == nil {
		return nil
	}
	return &MissingSessionCwdError{Issue: *issue}
}
