package main

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"syscall"

	jev "github.com/kazz187/jev-sdk-go"
	"github.com/spf13/cobra"
	"github.com/tylergannon/gimbal/internal/claimaudit"
)

func newAuditIndexCommand(stdout io.Writer) *cobra.Command {
	var dir string
	var prepare bool
	var pass int
	command := &cobra.Command{Use: "audit-index", Short: "Audit every indexed claim against local sources and all claim pairs", Long: `Prepare inventories research INDEX.md and clips and assigns block IDs for claim extraction.
The curator writes .semantic-index/claims.jsonl using inventory.json. The normal
invocation checks extraction, each original citation, and every pair with Jev;
it writes audit.jsonl, AUDIT.md, and completion.json. A complete audit may
contain findings and require research repair before authoring. Exit 65 asks the
curator to repair invalid claim records; exit 75 permits a transport retry. Completion also
reports an o200k_base proxy for a whole-index Jev packet; this diagnostic is
not a Jev token count and does not skip the exhaustive pair pass.

Example: gimbal audit-index --prepare --research-dir /absolute/research
         gimbal audit-index --research-dir /absolute/research`, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		if dir == "" {
			return fmt.Errorf("--research-dir is required")
		}
		if prepare {
			inv, err := claimaudit.Prepare(dir)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(stdout, "prepared %d index blocks; extract %d blocks in %s/.semantic-index/inventory.json\n", len(inv.Blocks), len(inv.Extract), dir)
			return err
		}
		if pass < 0 {
			return fmt.Errorf("--repair-pass must be nonnegative")
		}
		if _, err := claimaudit.Begin(dir, pass); err != nil {
			return err
		}
		// The command writes the incomplete current-revision summary before secret
		// loading, so direct invocation can resume after a transient loader error.
		if strings.TrimSpace(os.Getenv("TYPESAFE_API_KEY")) == "" {
			if err := loadCommandSecrets(cmd.Context()); err != nil {
				if transientSecretError(err) {
					return &claimaudit.TransientError{Err: err}
				}
				return err
			}
		}
		client, err := jev.New(jev.WithModel("jev-1.13.0"))
		if err != nil {
			return err
		}
		result, err := claimaudit.Audit(cmd.Context(), dir, client, pass)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(stdout, "audit revision %s: %d claims, %d/%d pairs, %d source findings, %d pair findings, whole-index proxy=%d o200k tokens (24k margin=%t; Jev screen untested), authoring_allowed=%t; %s/.semantic-index/completion.json\n", result.Revision, result.Metrics.Claims, result.Metrics.PairsAnswered, result.Metrics.PairsTotal, result.Metrics.SourceFindings, result.Metrics.PairFindings, result.Metrics.WholeIndexO200kTokens, result.Metrics.WholeIndexUnder24kProxy, result.AuthoringAllowed, dir)
		return err
	}}
	command.Flags().StringVar(&dir, "research-dir", "", "absolute research directory")
	command.Flags().BoolVar(&prepare, "prepare", false, "inventory research blocks without a Jev key")
	command.Flags().IntVar(&pass, "repair-pass", 0, "number of curator repairs preceding this audit")
	return command
}

func transientSecretError(err error) bool {
	if netErr, ok := errors.AsType[net.Error](err); ok {
		return netErr.Timeout() || errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.EPIPE)
	}
	var status *httpStatusError
	return errors.As(err, &status) && (status.Code == http.StatusTooManyRequests || status.Code >= 500)
}
