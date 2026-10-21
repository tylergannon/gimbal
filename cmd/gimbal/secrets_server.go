package main

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/tylergannon/gimbal/internal/secretshttp"
)

func newSecretsServerCommand() *cobra.Command {
	var directory, identityFile, tokenFile, listen string
	command := &cobra.Command{
		Use:   "secrets-server",
		Short: "Serve workflow secrets from an age-encrypted file over authenticated HTTP",
		Long: `Serve workflow secrets from an age-encrypted file over authenticated HTTP.
The identity file contains one native age X25519 private key (age-keygen).
The token file contains a separately generated random bearer credential of at
least 32 characters. Provision that token file on every trusted execution host.
The server alone needs the age identity. Both files should have mode 0600.
HTTP carries credentials and values in plaintext: keep this listener on a trusted
network, or put it behind a TLS proxy. No TLS configuration is required.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			token, err := secretshttp.ReadToken(tokenFile)
			if err != nil {
				return err
			}
			store, err := secretshttp.Open(directory, identityFile)
			if err != nil {
				return err
			}
			listener, err := net.Listen("tcp", listen)
			if err != nil {
				return err
			}
			defer listener.Close()
			server := &http.Server{Handler: secretshttp.Handler(store, token), ReadHeaderTimeout: 10 * time.Second}
			ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer cancel()
			stopped := make(chan struct{})
			defer close(stopped)
			go func() {
				select {
				case <-ctx.Done():
					_ = server.Close()
				case <-stopped:
				}
			}()
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Secrets listening on http://%s\n", listener.Addr()); err != nil {
				return err
			}
			err = server.Serve(listener)
			if errors.Is(err, http.ErrServerClosed) {
				return nil
			}
			return err
		},
	}
	command.Flags().StringVar(&directory, "directory", "", "absolute directory for encrypted secrets data")
	command.Flags().StringVar(&identityFile, "identity-file", "", "absolute path to the server's age identity")
	command.Flags().StringVar(&tokenFile, "token-file", "", "absolute path to the bearer credential")
	command.Flags().StringVar(&listen, "listen", "127.0.0.1:8082", "HTTP listen address")
	for _, name := range []string{"directory", "identity-file", "token-file"} {
		_ = command.MarkFlagRequired(name)
	}
	return command
}
