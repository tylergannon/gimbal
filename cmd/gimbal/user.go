package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tylergannon/gimbal/auth"
)

func newUserCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "user",
		Short: "Provision or remove users through a running instance's local control socket",
		Long: `Manage trusted web operators through the running instance's Unix control
socket. Authentication must be enabled for that instance. Login never creates
users. For GitHub, obtain the numeric subject with gh api users/LOGIN --jq .id;
the GitHub login name is display text, not the stored identity key.`,
		Args: cobra.NoArgs,
	}
	var addDir, issuer, subject, name string
	add := &cobra.Command{
		Use:     "add",
		Short:   "Provision a verified provider identity and print its stable local user ID",
		Example: "  gimbal user add --instance-dir /absolute/instance --issuer https://github.com --subject 123456 --name Tyler",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(issuer) == "" || strings.TrimSpace(subject) == "" || strings.TrimSpace(name) == "" {
				return errors.New("user add requires --issuer, --subject, and --name")
			}
			runtime, err := userInstance(cmd.Context(), addDir)
			if err != nil {
				return err
			}
			body, err := json.Marshal(auth.Identity{Issuer: issuer, Subject: subject, Name: name})
			if err != nil {
				return err
			}
			response, cleanup, err := runtime.request(cmd.Context(), http.MethodPost, "/control/users", bytes.NewReader(body))
			if err != nil {
				return err
			}
			defer cleanup()
			defer func() { _ = response.Body.Close() }()
			if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusCreated {
				return userResponseError(response)
			}
			var user struct {
				ID string `json:"id"`
			}
			if err := json.NewDecoder(response.Body).Decode(&user); err != nil {
				return err
			}
			if user.ID == "" {
				return errors.New("instance returned an empty user ID")
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), user.ID)
			return err
		},
	}
	add.Flags().StringVar(&addDir, "instance-dir", "", "absolute directory of the running instance (required)")
	add.Flags().StringVar(&issuer, "issuer", "", "verified provider issuer (required; GitHub: https://github.com)")
	add.Flags().StringVar(&subject, "subject", "", "immutable provider user ID (required; GitHub: numeric ID)")
	add.Flags().StringVar(&name, "name", "", "display name (required)")
	var removeDir string
	remove := &cobra.Command{
		Use:   "remove USER_ID",
		Short: "Remove a local user and revoke their access",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(args[0]) == "" {
				return errors.New("user remove requires a nonempty user ID")
			}
			runtime, err := userInstance(cmd.Context(), removeDir)
			if err != nil {
				return err
			}
			response, cleanup, err := runtime.request(cmd.Context(), http.MethodDelete, "/control/users/"+url.PathEscape(args[0]), nil)
			if err != nil {
				return err
			}
			defer cleanup()
			defer func() { _ = response.Body.Close() }()
			if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusNoContent {
				return userResponseError(response)
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), args[0])
			return err
		},
	}
	remove.Flags().StringVar(&removeDir, "instance-dir", "", "absolute directory of the running instance (required)")
	command.AddCommand(add, remove)
	return command
}

func userResponseError(response *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
	return fmt.Errorf("user operation: instance returned %s: %s", response.Status, strings.TrimSpace(string(body)))
}

// Discovery files are hints. Probe the local socket before using the selected
// instance, and refuse ambiguous live owners instead of choosing arbitrarily.
func userInstance(ctx context.Context, dir string) (runtimeClient, error) {
	if dir == "" || !filepath.IsAbs(dir) {
		return runtimeClient{}, errors.New("user operation requires --instance-dir with an absolute path")
	}
	entries, err := os.ReadDir(filepath.Join(dir, "control"))
	if err != nil {
		return runtimeClient{}, fmt.Errorf("discover instance: %w", err)
	}
	var live []runtimeClient
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, "control", entry.Name()))
		if err != nil {
			continue
		}
		var discovery runtimeDiscovery
		if json.Unmarshal(raw, &discovery) != nil || discovery.PID <= 0 || !filepath.IsAbs(discovery.Socket) {
			continue
		}
		runtime := runtimeClient{pid: discovery.PID, socket: discovery.Socket}
		probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		response, cleanup, err := runtime.request(probeCtx, http.MethodGet, "/control/live", nil)
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode == http.StatusOK {
				live = append(live, runtime)
			}
		}
		cleanup()
		cancel()
	}
	if err := ctx.Err(); err != nil {
		return runtimeClient{}, err
	}
	if len(live) != 1 {
		return runtimeClient{}, fmt.Errorf("expected one live Gimbal instance in %s, found %d", dir, len(live))
	}
	return live[0], nil
}
