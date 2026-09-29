package main

import (
	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/experiments/instrumented/continuity"
)

const coder gimbal.WorkflowRole = "coder"
const coach gimbal.WorkflowRole = "coach"

// Handwritten output uses the same result types as the authored source.
type Report = continuity.Report
type Checks = continuity.Checks
