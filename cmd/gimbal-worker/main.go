// gimbal-worker serves native tasks without workflow or plugin hosting.
package main

import (
	"os"

	"github.com/spf13/cobra"
)

func main() {
	if err := newWorkerCommand().Execute(); err != nil {
		os.Exit(1)
	}
}

type workerConfig struct {
	address, workdir, stateDir, envFile string
	service                             bool
	serviceName                         string
}

func newWorkerCommand() *cobra.Command {
	var config workerConfig
	command := &cobra.Command{
		Use:   "gimbal-worker",
		Short: "Serve native tasks in the foreground or as a Windows SCM service",
		Long: `Run the existing Gimbal task HTTP protocol without hosting workflows or plugins.
This executable supplies native execution and service hosting. Its HTTP listener
is a worker transport. For direct worker qualification, --task-endpoint on the
coordinator selects this one worker for the run's tasks. That invocation does
not provide distributed task placement across workers. Workflow requirements
and deployment transport addresses are separate concerns.

Commands and agents use --workdir on this machine, overriding coordinator paths.
Provision that workspace and provider CLIs/configuration for the worker account.

The default mode is a foreground console worker; stop with Ctrl+C. On Windows,
--service connects to SCM as --service-name (default gimbal-worker). Install an
own-process service with these flags in its binary path, under the provisioned
user account with Log on as a service. Account/password setup and installation
are external; this command does not install a service. Use absolute --workdir,
--executor-state-dir and --env-file paths in service mode. Running means the
state was restored and the HTTP listener opened, not provider authentication.
SCM Stop/Shutdown closes the bridge, preserving independent task owners. Owners
survive worker restart, not Windows shutdown; reuse the same state directory.

--env-file reads JSON: {"set":{"NAME":"literal value"},"user":["NAME"]}.
Set values override the process environment without variable expansion. On
Windows, user imports named string values from HKCU\Environment of the service
account (REG_SZ only); a missing/empty import fails startup. For example, import the existing
DIFFUSION_API_KEY by name. Supply the user's full PATH, USERPROFILE, HOME,
APPDATA, LOCALAPPDATA, TEMP/TMP and any provider configuration locations needed.
SCM does not inherit your interactive shell. Use the existing user's login
stores; never put credentials in service arguments. Restrict the file to that
user, SYSTEM and administrators; environment values are never printed. Changes
apply on worker restart to new owners; existing owners retain their environment.

The endpoint executes trusted requests without authentication. Bind to loopback
or a protected private interface. Artifacts travel through the task byte endpoint.`,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error { return runHost(command, config) },
	}
	command.Flags().StringVar(&config.address, "listen", "127.0.0.1:8081", "trusted task HTTP listen address")
	command.Flags().StringVar(&config.workdir, "workdir", ".", "provisioned local workspace for all tasks")
	command.Flags().StringVar(&config.stateDir, "executor-state-dir", "", "persistent executor state directory (default ~/.gimbal/executors)")
	command.Flags().StringVar(&config.envFile, "env-file", "", "explicit environment JSON file (required with --service)")
	command.Flags().BoolVar(&config.service, "service", false, "run under Windows SCM instead of foreground signal handling")
	command.Flags().StringVar(&config.serviceName, "service-name", "gimbal-worker", "installed Windows SCM service name")
	return command
}
