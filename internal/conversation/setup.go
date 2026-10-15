package conversation

import (
	"context"
	"errors"
	"github.com/tylergannon/gimbal/internal/environments"
)

// SetupEnvironment observes choices on the selected machine without starting a
// harness. A stopped registered route may require a bounded metadata probe.
func (m *Manager) SetupEnvironment(ctx context.Context, environment string) (Setup, error) {
	if environment == "" {
		environment = environments.Local
	}
	m.mu.RLock()
	service, projectID := m.environments, m.projectID
	m.mu.RUnlock()
	if environment == environments.Local {
		setup, err := m.Setup(ctx)
		if err != nil {
			return setup, err
		}
		setup.Environment, setup.Machine, setup.Capabilities = environments.Local, "This Mac", localCapabilities()
		if service != nil {
			snapshot, err := service.Snapshot(ctx, projectID)
			if err != nil {
				return setup, err
			}
			setup.Environments = snapshot
		}
		return setup, nil
	}
	if service == nil {
		return Setup{}, errors.New("conversation: environment service unavailable")
	}
	target, err := service.Resolve(ctx, projectID, environment)
	if err != nil {
		return Setup{}, err
	}
	directory, err := service.Inspect(ctx, target, target.Directory)
	if err != nil {
		return Setup{}, err
	}
	snapshot, err := service.Snapshot(ctx, projectID)
	if err != nil {
		return Setup{}, err
	}
	setup := Setup{Harnesses: []Harness{}, Directory: directory.Path, Git: directory.Git && !directory.Unborn, Branch: directory.Branch, Unborn: directory.Unborn, Bases: directory.Bases, Base: directory.Base, Environment: environment, Machine: target.Machine.Name, Capabilities: target.Capabilities, Environments: snapshot, HarnessStates: []environments.Harness{}}
	for _, machine := range snapshot.Machines {
		if machine.ID != target.Machine.ID {
			continue
		}
		setup.Capabilities.Steer = machine.Capabilities.Steer
		setup.Capabilities.Fork = machine.Capabilities.Fork
		setup.HarnessStates = machine.Harnesses
		for _, harness := range machine.Harnesses {
			if _, _, supported := providerModel(harness.ID, harness.Default); supported == nil && harness.State != environments.HarnessMissing && harness.State != environments.HarnessSignInRequired {
				setup.Harnesses = append(setup.Harnesses, Harness{ID: harness.ID, Default: harness.Default, Models: harness.Models})
			}
		}
	}
	setup.Capabilities.Worktree = directory.Git && !directory.Unborn && directory.FixedWorkdir == ""
	return setup, nil
}
