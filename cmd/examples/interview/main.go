// Command interview runs the human interview example in this process.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/binding"
	interviewworkflow "github.com/tylergannon/gimbal/internal/workflows/interview"
	"github.com/tylergannon/gimbal/web"
)

func main() {
	projectFlag := flag.String("project", ".", "project directory for the standalone run")
	workDirFlag := flag.String("work-dir", "", "execution directory (default: project)")
	topic := flag.String("topic", "", "topic for the interview (required)")
	model := flag.String("interviewer", "gpt-5.6-terra", "model for the interviewer role")
	port := flag.Int("port", 8081, "loopback web port for answering the interview")
	flag.Parse()
	if *topic == "" || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "interview: give --topic and no positional arguments")
		os.Exit(2)
	}
	project, err := filepath.Abs(*projectFlag)
	if err == nil {
		workDir := *workDirFlag
		if workDir == "" {
			workDir = project
		}
		workDir, err = filepath.Abs(workDir)
		if err == nil {
			var models map[gimbal.WorkflowRole]gimbal.ModelBinding
			models, err = binding.Roles(map[gimbal.WorkflowRole]string{"interviewer": *model})
			if err == nil {
				ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
				defer stop()
				var instance *web.Instance
				instance, err = web.NewInstance(ctx, filepath.Join(project, ".gimbal"), []string{project}, web.WithPort(*port))
				if err == nil {
					runtime, projectErr := instance.Owner.Project(project)
					if projectErr != nil {
						err = projectErr
					} else {
						fmt.Printf("Answer the interview at http://127.0.0.1:%d/ (open the project and its live run).\n", *port)
						err = runtime.Run(ctx, "interview", models, func(ctx context.Context) error {
							return interviewworkflow.Interview(ctx, gimbal.Env{WorkDir: workDir}, interviewworkflow.InterviewParams{Topic: *topic})
						})
						if err == nil {
							fmt.Println("Interview complete. The page remains available until Ctrl+C.")
							<-ctx.Done()
						}
					}
				}
			}
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "interview:", err)
		os.Exit(1)
	}
}
