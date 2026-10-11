package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/invocation"
	"github.com/tylergannon/gimbal/web"
)

// newRunCommand discovers commands from the selected central before parsing
// workflow flags. Control clients never open local plugin files.
func newRunCommand() *cobra.Command {
	return &cobra.Command{Use: "run [workflow]", Short: "Submit a workflow from the selected central's catalog", DisableFlagParsing: true, Args: cobra.ArbitraryArgs, RunE: func(cmd *cobra.Command, args []string) error {
		instance := os.Getenv("GIMBAL_INSTANCE_DIR")
		if instance == "" {
			instance = ".gimbal"
		}
		project := "."
		for i, arg := range args {
			for flag, target := range map[string]*string{"--instance-dir": &instance, "--project": &project} {
				if arg == flag && i+1 < len(args) {
					*target = args[i+1]
				}
				if after, ok := strings.CutPrefix(arg, flag+"="); ok {
					*target = after
				}
			}
		}
		catalog, err := web.WorkflowCatalog(cmd.Context(), instance, project)
		if err != nil {
			if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
				return cmd.Help()
			}
			return err
		}
		run := &cobra.Command{Use: "run", Short: "Submit a registered workflow", SilenceUsage: true, SilenceErrors: true, Args: cobra.NoArgs}
		run.Flags().String("instance-dir", instance, "selected central state directory")
		run.Flags().String("project", project, "central project directory")
		run.SetOut(cmd.OutOrStdout())
		run.SetErr(cmd.ErrOrStderr())
		for _, description := range catalog {
			run.AddCommand(catalogCommand(description))
		}
		run.SetArgs(args)
		return run.ExecuteContext(cmd.Context())
	}}
}

func catalogCommand(d gimbal.WorkflowDescription) *cobra.Command {
	var input, project, workDir, instanceDir, conversation, taskEndpoint, workflowEndpoint, workflowProject, taskRoute, workflowRoute string
	var follow bool
	values := map[string]func() any{}
	roles := map[gimbal.WorkflowRole]*string{}
	cmd := &cobra.Command{Use: d.Name, Short: d.Summary, Long: d.Help + "\n\nEach accepted run executes in an independent process that survives after the client exits and through central restart. Native executables and provider configuration come from the instance startup environment, or the selected task backend. Each role flag chooses a model and optional effort.", Args: cobra.NoArgs}
	for _, field := range d.Fields {
		usage := field.Description
		if !field.Optional {
			usage += " (required)"
		}
		switch field.Kind {
		case "string":
			v := new(string)
			cmd.Flags().StringVar(v, field.Flag, "", usage)
			values[field.Name] = func() any { return *v }
		case "int":
			v := new(int)
			cmd.Flags().IntVar(v, field.Flag, 0, usage)
			values[field.Name] = func() any { return *v }
		case "bool":
			v := new(bool)
			cmd.Flags().BoolVar(v, field.Flag, false, usage)
			values[field.Name] = func() any { return *v }
		}
	}
	for role, defaultModel := range d.Roles {
		v := new(string)
		cmd.Flags().StringVar(v, string(role), defaultModel, "advanced override for role "+string(role)+", as model or model:effort; omit this flag to use the displayed workflow default")
		roles[role] = v
	}
	cmd.Flags().StringVar(&input, "input", "", "JSON object or @file; cannot combine with input flags")
	cmd.Flags().StringVar(&project, "project", ".", "central project directory")
	cmd.Flags().StringVar(&workDir, "work-dir", "", "execution environment workdir")
	instanceDefault := os.Getenv("GIMBAL_INSTANCE_DIR")
	if instanceDefault == "" {
		instanceDefault = ".gimbal"
	}
	cmd.Flags().StringVar(&instanceDir, "instance-dir", instanceDefault, "selected central state directory")
	cmd.Flags().StringVar(&conversation, "conversation", "", "associate with a project conversation")
	cmd.Flags().StringVar(&taskEndpoint, "task-endpoint", "", "task backend HTTP endpoint")
	cmd.Flags().StringVar(&workflowEndpoint, "workflow-endpoint", "", "workflow backend HTTP endpoint")
	cmd.Flags().StringVar(&taskRoute, "task-route", "", "task backend route as JSON or @file (name, configuration, environment)")
	cmd.Flags().StringVar(&workflowRoute, "workflow-route", "", "workflow backend route as JSON or @file (name, configuration, environment)")
	cmd.Flags().StringVar(&workflowProject, "workflow-project", "", "project directory at workflow backend")
	cmd.Flags().BoolVar(&follow, "follow", false, "wait for completion")
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		var err error
		project, err = filepath.Abs(project)
		if err != nil {
			return err
		}
		if workDir == "" {
			workDir = project
		}
		submitted := map[string]any{}
		for _, f := range d.Fields {
			if cmd.Flags().Changed(f.Flag) || (f.Kind == "bool" && !f.Optional && input == "") {
				submitted[f.Name] = values[f.Name]()
			}
		}
		if input != "" && len(submitted) > 0 {
			return fmt.Errorf("--input cannot be combined with parameter flags")
		}
		raw := json.RawMessage(input)
		if after, ok := strings.CutPrefix(input, "@"); ok {
			raw, err = os.ReadFile(after)
			if err != nil {
				return err
			}
		}
		if input == "" {
			raw, err = json.Marshal(submitted)
			if err != nil {
				return err
			}
		}
		overrides := map[gimbal.WorkflowRole]string{}
		for role, v := range roles {
			if cmd.Flags().Changed(string(role)) {
				overrides[role] = *v
			}
		}
		request := invocation.Request{Workflow: d.Name, Input: raw, Project: project, WorkDir: workDir, Conversation: conversation, Roles: overrides, TaskEndpoint: taskEndpoint}
		if workflowEndpoint != "" {
			config, _ := json.Marshal(map[string]string{"endpoint": workflowEndpoint})
			request.WorkflowRoute = backend.Route{Name: "http", Configuration: config, Environment: workflowEndpoint}
			request.WorkflowProject = workflowProject
		}
		if taskRoute != "" {
			if taskEndpoint != "" {
				return fmt.Errorf("--task-route cannot be combined with --task-endpoint")
			}
			route, err := readRoute(taskRoute)
			if err != nil {
				return err
			}
			request.TaskRoute = route
		}
		if workflowRoute != "" {
			if workflowEndpoint != "" {
				return fmt.Errorf("--workflow-route cannot be combined with --workflow-endpoint")
			}
			route, err := readRoute(workflowRoute)
			if err != nil {
				return err
			}
			request.WorkflowRoute = route
			request.WorkflowProject = workflowProject
		}
		accepted, err := web.Start(cmd.Context(), instanceDir, request)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintln(cmd.OutOrStdout(), accepted.RunID); err != nil {
			return err
		}
		if !follow {
			return nil
		}
		result, err := web.Follow(cmd.Context(), instanceDir, project, accepted.RunID)
		if err != nil {
			return err
		}
		if result.Status != "completed" {
			return fmt.Errorf("run %s %s: %s", accepted.RunID, result.Status, result.Error)
		}
		return nil
	}
	return cmd
}

func readRoute(value string) (backend.Route, error) {
	raw := []byte(value)
	if strings.HasPrefix(value, "@") {
		var err error
		raw, err = os.ReadFile(strings.TrimPrefix(value, "@"))
		if err != nil {
			return backend.Route{}, err
		}
	}
	var route backend.Route
	if err := json.Unmarshal(raw, &route); err != nil {
		return route, err
	}
	if route.Name == "" {
		return route, fmt.Errorf("backend route requires name")
	}
	return route, nil
}
