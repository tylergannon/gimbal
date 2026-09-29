package main

import (
	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/experiments/instrumented/continuity"
)

const coder gimbal.WorkflowRole = "coder"
const coach gimbal.WorkflowRole = "coach"

// Shared tests and the handwritten fanout use the authored result types.
type Report = continuity.Report
type Checks = continuity.Checks
