// Package app declares the application-owned request state used by SKGo.
package app

import "github.com/tylergannon/skgo"

// Locals is empty because Gimbal services are supplied by the runtime context.
type Locals struct{}

type RequestEvent[P any] = skgo.RequestEvent[P, Locals]
