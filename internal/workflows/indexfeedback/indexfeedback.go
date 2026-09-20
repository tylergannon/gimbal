// Package indexfeedback gives advisory feedback on a semantic index using
// three sampled questions and fresh retrieval sessions. It reads local sources
// only, makes no repairs, and does not certify or gate the originating workflow.
//
// A source reader samples three questions and supporting passages, a fresh
// retriever answers each using the index, and an assessor checks the answers
// against original evidence. All three roles default to GPT Luna. The whole
// evaluation has a ten-minute deadline, including a two-minute limit per
// retrieval. Errors leave an incomplete feedback file and a failed sidecar run.
//
// Feedback is written to --output and recorded in the sidecar's run. It includes
// observed retrieval calls, recorded returned-text bytes, elapsed time, and
// provider token accounting when available. These are sampled observations,
// not exact physical file reads or a reproducible benchmark. The index and
// corpus are read in place; concurrent updates may affect the observations.
// research-document records the feedback location and launches this command
// independently after index creation and updates, without waiting for it.
//
// Example:
//
//	gimble run index-feedback --no-web --goal "Explain service lifetimes" \
//	  --corpus /abs/research --index /abs/research/INDEX.md \
//	  --output /abs/feedback.md
package indexfeedback

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/tylergannon/gimble"
)

//go:generate go tool polytype --validate
//go:generate go run github.com/tylergannon/gimble/internal/generate/gimblegen -entry IndexFeedback -name index-feedback

type Params struct {
	// Goal is the task the index should help an agent accomplish.
	Goal string
	// Corpus is the local source directory; it is read only.
	Corpus string
	// Index is the existing semantic-index entrypoint.
	Index string
	// Output receives advisory Markdown feedback, including incomplete outcomes.
	Output string
}

type Sample struct {
	Question string `json:"question"`
	// Evidence gives original source paths, line ranges, and supporting excerpts.
	Evidence string `json:"evidence"`
}

type Questions struct {
	Questions []Sample `json:"questions"`
}

// IndexFeedback samples retrieval quality and posts advisory feedback.
func IndexFeedback(ctx context.Context, env gimble.Env, params Params) (err error) {
	if strings.TrimSpace(params.Goal) == "" {
		return fmt.Errorf("goal must not be blank")
	}
	for _, path := range []*string{&params.Corpus, &params.Index, &params.Output} {
		if strings.TrimSpace(*path) == "" {
			return fmt.Errorf("corpus, index, and output are required")
		}
		if !filepath.IsAbs(*path) {
			*path = filepath.Join(env.WorkDir, *path)
		}
		*path = filepath.Clean(*path)
	}
	if rel, e := filepath.Rel(params.Corpus, params.Output); e == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("feedback output must be outside the read-only corpus")
	}
	if err := os.MkdirAll(filepath.Dir(params.Output), 0o755); err != nil {
		return err
	}
	var attempts strings.Builder
	defer func() {
		if err != nil {
			err = errors.Join(err, os.WriteFile(params.Output, []byte("# Index feedback — incomplete\n\n"+err.Error()+"\n\n"+attempts.String()), 0o644))
		}
	}()
	if err := os.WriteFile(params.Output, []byte("# Index feedback — running\n\nAdvisory evaluation in progress. See the index-feedback run for activity.\n"), 0o644); err != nil {
		return err
	}
	if info, e := os.Stat(params.Corpus); e != nil || !info.IsDir() {
		return fmt.Errorf("corpus is not a readable directory: %s", params.Corpus)
	}
	if info, e := os.Stat(params.Index); e != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		return fmt.Errorf("index is not a nonempty file: %s", params.Index)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	// A neutral working directory avoids inheriting the builder's repository
	// instructions. Answers and evaluation records are not placed here.
	workdir, err := os.MkdirTemp("", "gimble-index-feedback-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(workdir) }()
	id := ulid.Make().String()
	gimble.Set(ctx, "evaluation id", id)
	gimble.Set(ctx, "goal", params.Goal)
	gimble.Set(ctx, "corpus", params.Corpus)
	gimble.Set(ctx, "index", params.Index)
	gimble.Set(ctx, "feedback file", params.Output)

	sampler := gimble.NewSession(ctx, "index-sampling", workdir)
	questions, err := sampler.Generate[Questions](ctx, samplePrompt, gimble.WithScopeTemplate(sampleContext))
	if err != nil {
		return err
	}
	if len(questions.Questions) != 3 {
		return fmt.Errorf("source sampler returned %d questions, need three", len(questions.Questions))
	}
	for _, sample := range questions.Questions {
		if strings.TrimSpace(sample.Question) == "" || strings.TrimSpace(sample.Evidence) == "" {
			return fmt.Errorf("sampled questions need a question and source evidence")
		}
	}
	runDir, accountingErr := findRun(filepath.Join(env.WorkDir, ".gimble", "runs"), id)
	number := 0
	for ctx, sample := range gimble.Iterate(ctx, "question", questions.Questions) {
		number++
		gimble.Set(ctx, "question", sample.Question)
		retriever := gimble.NewSession(ctx, "index-retrieval", workdir)
		retrievalCtx, stop := context.WithTimeout(ctx, 2*time.Minute)
		started := time.Now()
		answer, retrievalErr := retriever.Generate[gimble.Text](retrievalCtx, retrievePrompt, gimble.WithScopeTemplate(retrievalContext))
		elapsed := time.Since(started)
		stop()
		measurement := "Accounting unavailable: " + fmt.Sprint(accountingErr)
		if accountingErr == nil {
			measurement = measureRetrieval(runDir, fmt.Sprintf("question.%d", number))
		}
		fmt.Fprintf(&attempts, "## Question %d\n\n%s\n\nAnswer:\n%s\n\nElapsed: %s\n\n%s\n\n", number, sample.Question, answer, elapsed.Round(time.Millisecond), measurement)
		if retrievalErr != nil {
			fmt.Fprintf(&attempts, "Retrieval incomplete: %s\n\n", retrievalErr)
		}
		gimble.Set(ctx, "retrieval accounting", measurement)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	// Expected evidence is only shared after all fresh retrievals have ended.
	gimble.SetJSON(ctx, "sampled evidence", questions)
	gimble.Set(ctx, "retrieval attempts", attempts.String())
	assessor := gimble.NewSession(ctx, "index-assessment", workdir)
	feedback, err := assessor.Generate[gimble.Text](ctx, assessPrompt, gimble.WithScopeTemplate(assessmentContext))
	if err != nil {
		return err
	}
	gimble.Set(ctx, "index feedback", feedback)
	report := "# Index feedback — advisory\n\n" + string(feedback) + "\n\n" + attempts.String()
	if runDir != "" {
		report += "Run records: " + runDir + "\n"
	}
	return os.WriteFile(params.Output, []byte(report), 0o644)
}

const sampleContext = `Goal: {{.By.goal.Text}}
Read-only corpus: {{.By.corpus.Text}}`

const retrievalContext = `Question: {{.By.question.Text}}
Index entrypoint: {{.By.index.Text}}
Read-only corpus: {{.By.corpus.Text}}`

const assessmentContext = `Goal: {{.By.goal.Text}}
Read-only corpus: {{.By.corpus.Text}}
Index entrypoint: {{.By.index.Text}}
Sampled evidence: {{(index .By "sampled evidence").Text}}
Retrieval attempts: {{(index .By "retrieval attempts").Text}}`

const samplePrompt = `Sample a few original source files in the local corpus and propose exactly three useful questions for the goal, with precise supporting source paths, line ranges, and short verbatim excerpts. Derive them from source evidence, not the index's claims. Keep this small: sample rather than exhaustively survey. Use only local source files; no outside research, other agents, edits, or evaluation records. Return the questions and evidence directly, without writing files.`

const retrievePrompt = `Answer the question using the semantic index as your starting point. Follow its routes to original evidence and give precise local source citations and short supporting excerpts. If you cannot find support, say what is missing. Keep this a quick attempt: aim for at most five retrieval calls and bounded excerpts rather than dumping whole files. Use only the supplied index and corpus; no outside research, other agents, edits, or evaluation records. Return your answer directly without writing files.`

const assessPrompt = `Give practical advisory feedback on this semantic index in at most 250 words. Check the returned citations and sampled evidence against original local source passages; the sampler can be wrong. Give one or two sentences per question distinguishing supported, unsupported, and incomplete answers. Then suggest up to three concrete routing improvements with affected paths, where the observations justify them. Do not blame the index for a retriever or harness failure or infer a universal quality score. The workflow will append the recorded measurements. Use only the supplied corpus and index, make no edits or files, and do not launch other agents. Return the feedback directly.`
