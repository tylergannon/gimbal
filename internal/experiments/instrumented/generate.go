package main

//go:generate go -C ../../../examples/temporal run ./cmd/generate -dir ../../internal/experiments/instrumented/continuity -entry Continuity -name continuity -output ../../internal/experiments/instrumented/continuity_temporal_gen.go
//go:generate go -C ../../../examples/temporal run ./cmd/generate -dir ../../internal/experiments/instrumented/planning -entry Planning -name planning -output ../../internal/experiments/instrumented/planning_temporal_gen.go
//go:generate go -C ../../../examples/temporal run ./cmd/generate -dir ../../internal/experiments/instrumented/resulttypes -entry Results -name results -output ../../internal/experiments/instrumented/results_temporal_gen.go
