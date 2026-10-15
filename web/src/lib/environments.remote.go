package workspace

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/tylergannon/gimbal/internal/app"
	"github.com/tylergannon/gimbal/internal/environments"
	"github.com/tylergannon/gimbal/internal/host"
	"github.com/tylergannon/gimbal/internal/skgo/params"
	"github.com/tylergannon/skgo"
)

func environmentService(ctx context.Context) (*environments.Service, error) {
	owner := host.OwnerFrom(ctx)
	if owner == nil {
		return nil, skgo.Errorf(503, "The desktop environment service is unavailable.")
	}
	return owner.Environments()
}

func environmentSnapshot(ctx context.Context, projectID string) (environments.Snapshot, error) {
	service, err := environmentService(ctx)
	if err != nil {
		return environments.Snapshot{}, err
	}
	return service.Snapshot(ctx, projectID)
}

func watchEnvironments(ctx context.Context, projectID string, yield func(environments.Snapshot) error) error {
	service, err := environmentService(ctx)
	if err != nil {
		return err
	}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	var previous string
	for {
		snapshot, err := service.Snapshot(ctx, projectID)
		if err != nil {
			return err
		}
		data, err := json.Marshal(snapshot)
		if err != nil {
			return err
		}
		if string(data) != previous {
			if err := yield(snapshot); err != nil {
				return err
			}
			previous = string(data)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func environmentConfig(ctx context.Context) (environments.ConfigDocument, error) {
	service, err := environmentService(ctx)
	if err != nil {
		return environments.ConfigDocument{}, err
	}
	return service.Config(ctx)
}

func sshHosts(ctx context.Context) ([]string, error) { return environments.SSHHosts(ctx) }

func watchMachineConnection(ctx context.Context, id string, yield func(environments.ConnectionProgress) error) error {
	service, err := environmentService(ctx)
	if err != nil {
		return err
	}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	var previous string
	for {
		progress := service.Connection(id)
		data, err := json.Marshal(progress)
		if err != nil {
			return err
		}
		if string(data) != previous {
			if err := yield(progress); err != nil {
				return err
			}
			previous = string(data)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func saveEnvironmentConfig(ctx context.Context, event app.RequestEvent[params.Params], edit environments.ConfigEdit) (environments.ConfigDocument, error) {
	service, err := environmentService(ctx)
	if err != nil {
		return environments.ConfigDocument{}, err
	}
	out, err := service.SaveConfig(ctx, edit)
	if err != nil {
		return out, skgo.Errorf(409, "%s", err)
	}
	return out, nil
}

// MachineForm intentionally leaves opaque route/plugin configuration in the file.
type MachineForm struct {
	Revision      string `json:"revision"`
	ID            string `json:"id"`
	Name          string `json:"name"`
	Kind          string `json:"kind"`
	Host          string `json:"host"`
	Port          int    `json:"port"`
	DaemonPort    int    `json:"daemon_port"`
	WorkspaceRoot string `json:"workspace_root"`
	Address       string `json:"address"`
	Executable    string `json:"executable"`
	ConnectAnyway bool   `json:"connect_anyway"`
}

func saveMachine(ctx context.Context, event app.RequestEvent[params.Params], in MachineForm) (environments.ConnectionProgress, error) {
	service, err := environmentService(ctx)
	if err != nil {
		return environments.ConnectionProgress{}, err
	}
	machine := environments.Machine{ID: strings.TrimSpace(in.ID), Name: strings.TrimSpace(in.Name), Kind: in.Kind, Executable: strings.TrimSpace(in.Executable)}
	switch in.Kind {
	case environments.KindSSH:
		machine.SSH = &environments.SSH{Host: strings.TrimSpace(in.Host), Port: in.Port, DaemonPort: in.DaemonPort, WorkspaceRoot: strings.TrimSpace(in.WorkspaceRoot)}
	case environments.KindAddress:
		machine.Address = strings.TrimSpace(in.Address)
	default:
		return environments.ConnectionProgress{}, skgo.Invalidf("kind", "Configure plugin and queue routes in the configuration file.")
	}
	out, err := service.ConnectMachine(ctx, environments.MachineEdit{Revision: in.Revision, Machine: machine, ConnectAnyway: in.ConnectAnyway})
	if err != nil {
		return out, skgo.Errorf(400, "%s", err)
	}
	return out, nil
}

func reconnectMachine(ctx context.Context, event app.RequestEvent[params.Params], in string) (environments.MachineState, error) {
	service, err := environmentService(ctx)
	if err != nil {
		return environments.MachineState{}, err
	}
	return service.Reconnect(ctx, in)
}
func removeMachine(ctx context.Context, event app.RequestEvent[params.Params], in environments.MachineRemoval) (bool, error) {
	service, err := environmentService(ctx)
	if err != nil {
		return false, err
	}
	if err := service.RemoveMachine(ctx, in); err != nil {
		return false, skgo.Errorf(409, "%s", err)
	}
	return true, nil
}
func removeEnvironment(ctx context.Context, event app.RequestEvent[params.Params], in environments.EnvironmentRemoval) (bool, error) {
	service, err := environmentService(ctx)
	if err != nil {
		return false, err
	}
	if err := service.RemoveEnvironment(ctx, in); err != nil {
		return false, skgo.Errorf(409, "%s", err)
	}
	return true, nil
}

type EnvironmentForm struct {
	Revision  string `json:"revision"`
	ID        string `json:"id"`
	Project   string `json:"project"`
	Machine   string `json:"machine"`
	Directory string `json:"directory"`
}

func saveEnvironment(ctx context.Context, event app.RequestEvent[params.Params], in EnvironmentForm) (environments.EnvironmentRow, error) {
	service, err := environmentService(ctx)
	if err != nil {
		return environments.EnvironmentRow{}, err
	}
	if _, err := project(ctx, in.Project); err != nil {
		return environments.EnvironmentRow{}, err
	}
	out, err := service.SaveEnvironment(ctx, environments.EnvironmentEdit{Revision: in.Revision, Environment: environments.Environment{ID: in.ID, Project: in.Project, Machine: in.Machine, Directory: in.Directory}})
	if err != nil {
		return out, skgo.Errorf(400, "%s", err)
	}
	return out, nil
}

type EnvironmentLocation struct {
	Project   string `json:"project"`
	Machine   string `json:"machine"`
	Directory string `json:"directory"`
}

// A prospective directory is inspected without saving it or starting a session.
func inspectionTarget(ctx context.Context, service *environments.Service, in EnvironmentLocation) (environments.Target, error) {
	if _, err := project(ctx, in.Project); err != nil {
		return environments.Target{}, err
	}
	return service.ResolveDirectory(ctx, in.Project, in.Machine, in.Directory)
}
func inspectEnvironment(ctx context.Context, in EnvironmentLocation) (environments.Directory, error) {
	service, err := environmentService(ctx)
	if err != nil {
		return environments.Directory{}, err
	}
	target, err := inspectionTarget(ctx, service, in)
	if err != nil {
		return environments.Directory{}, err
	}
	return service.Inspect(ctx, target, in.Directory)
}
func listEnvironmentDirectories(ctx context.Context, in EnvironmentLocation) (environments.Directory, error) {
	service, err := environmentService(ctx)
	if err != nil {
		return environments.Directory{}, err
	}
	target, err := inspectionTarget(ctx, service, in)
	if err != nil {
		return environments.Directory{}, err
	}
	return service.ListDirectories(ctx, target, in.Directory)
}

type CloneEnvironment struct {
	Project   string `json:"project"`
	Machine   string `json:"machine"`
	Directory string `json:"directory"`
	Origin    string `json:"origin"`
	Operation string `json:"operation"`
}

func cloneEnvironment(ctx context.Context, event app.RequestEvent[params.Params], in CloneEnvironment) (environments.Directory, error) {
	service, err := environmentService(ctx)
	if err != nil {
		return environments.Directory{}, err
	}
	target, err := inspectionTarget(ctx, service, EnvironmentLocation{Project: in.Project, Machine: in.Machine, Directory: in.Directory})
	if err != nil {
		return environments.Directory{}, err
	}
	out, err := service.Clone(ctx, target, in.Origin, in.Directory, in.Operation)
	if err != nil {
		return out, skgo.Errorf(400, "%s", err)
	}
	return out, nil
}

var _ = skgo.LiveQuery(watchEnvironments)
var _ = skgo.LiveQuery(watchMachineConnection)
var _ = skgo.Query(environmentSnapshot)
var _ = skgo.Query(environmentConfig)
var _ = skgo.Query(sshHosts)
var _ = skgo.Form(saveEnvironmentConfig)
var _ = skgo.Form(saveMachine)
var _ = skgo.Command(reconnectMachine)
var _ = skgo.Command(removeMachine)
var _ = skgo.Command(removeEnvironment)
var _ = skgo.Form(saveEnvironment)
var _ = skgo.Query(inspectEnvironment)
var _ = skgo.Query(listEnvironmentDirectories)
var _ = skgo.Form(cloneEnvironment)
