// Package main registers the generated mixed-workflow fixture with the host.
package main

import (
	"github.com/tylergannon/gimbal"
	fixture "github.com/tylergannon/gimbal/internal/runner/testdata/distributedplugin"
)

func Register(r *gimbal.Registry) error { return r.RegisterWorkflow(fixture.MixedDefinition()) }
