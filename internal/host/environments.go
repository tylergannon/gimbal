package host

import (
	"errors"
	"fmt"
	"os"

	"github.com/tylergannon/gimbal/internal/environments"
)

// Environments returns the instance's shared machine configuration and
// placement service. Initialization errors remain visible to remote functions
// and project admission rather than producing a second, empty configuration.
func (o *Owner) Environments() (*environments.Service, error) {
	if o == nil {
		return nil, errors.New("gimbal: instance is unavailable")
	}
	o.environmentsOnce.Do(func() {
		cli, err := os.Executable()
		if err != nil {
			o.environmentsErr = fmt.Errorf("gimbal: locate environment executable: %w", err)
			return
		}
		o.environments, o.environmentsErr = environments.New(o.ctx, environments.Options{
			InstanceDir: o.dir,
			Executable:  cli,
			Sessions:    o.environmentSessions,
		})
	})
	return o.environments, o.environmentsErr
}

// environmentSessions reads the authoritative catalog and manager state. It
// never scans OS processes, and releases catalog locks before reading managers.
func (o *Owner) environmentSessions() []environments.SessionUsage {
	items := []environments.SessionUsage{}
	for _, project := range o.Projects() {
		for _, conversation := range project.Conversations().List() {
			machine, environment := environments.Local, environments.Local
			if conversation.Placement.Present {
				machine = conversation.Placement.Value.Machine.ID
				environment = conversation.Placement.Value.Environment.ID
			}
			items = append(items, environments.SessionUsage{
				Project: project.ID(), Conversation: conversation.ID, Title: conversation.Title,
				Machine: machine, Environment: environment, Directory: conversation.Worktree,
				Status: conversation.Status, Active: conversation.Live || conversation.ActiveRequest != "",
				Pending: conversation.CleanupPending || conversation.Status == "unresolved",
			})
		}
	}
	return items
}
