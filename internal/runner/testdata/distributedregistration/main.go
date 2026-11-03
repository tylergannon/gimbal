// Package main registers the generated mixed-workflow fixture with the host.
package main

import (
	"github.com/tylergannon/gimbal"
	fixture "github.com/tylergannon/gimbal/internal/runner/testdata/distributedplugin"
	restart "github.com/tylergannon/gimbal/internal/runner/testdata/restartplugin"
)

func Register(r *gimbal.Registry) error {
	if err := r.RegisterWorkflow(fixture.MixedDefinition()); err != nil {
		return err
	}
	return r.RegisterWorkflow(restart.RestartDefinition())
}
