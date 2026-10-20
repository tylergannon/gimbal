package conversation

import (
	"context"
	"errors"
	"github.com/tylergannon/gimbal/internal/environments"
	"slices"
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
			for _, machine := range snapshot.Machines {
				if machine.ID == environments.Local {
					setup.HarnessStates = machine.Harnesses
					setup.Harnesses = observedHarnesses(machine.Harnesses, setup.Harnesses)
				}
			}
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
		setup.Harnesses = observedHarnesses(machine.Harnesses, nil)
	}
	setup.Capabilities.Worktree = directory.Git && !directory.Unborn && directory.FixedWorkdir == ""
	return setup, nil
}

// observedHarnesses offers each machine harness's reported models as canonical
// selections, once each. A reported model that cannot run on that harness is
// not offered, and no model the machine did not report is ever added. When the
// reported default is not offered, the first offered model leads instead; a
// harness with no offerable model is left out of the choices, and its reported
// state still shows in HarnessStates.
func observedHarnesses(states []environments.Harness, fallback []Harness) []Harness {
	if len(states) == 0 {
		return fallback
	}
	result := []Harness{}
	for _, h := range states {
		models := []string{}
		for _, reported := range append([]string{h.Default}, h.Models...) {
			if offered, _, err := providerModel(h.ID, reported); reported != "" && err == nil && !slices.Contains(models, offered) {
				models = append(models, offered)
			}
		}
		if len(models) > 0 {
			result = append(result, harnessOptions(h.ID, models[0], models))
		}
	}
	return result
}
