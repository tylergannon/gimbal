package effects

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/tylergannon/gimbal/backend"
)

type connectionKey struct{}

func WithConnection(ctx context.Context, c backend.Connection) context.Context {
	return context.WithValue(ctx, connectionKey{}, c)
}
func Connection(ctx context.Context) backend.Connection {
	c, _ := ctx.Value(connectionKey{}).(backend.Connection)
	return c
}

// dispatch bridges the runtime's concrete request DTOs to the resolved plugin.
func dispatch(ctx context.Context, c backend.Connection, op string, in, out any) error {
	var value any
	var err error
	switch op {
	case "session":
		value, err = c.Execution.Session(ctx, in.(Session))
	case "turn":
		err = c.Execution.Turn(ctx, in.(Turn))
		value = Reply{}
	case "process":
		err = c.Execution.Process(ctx, in.(Process))
		value = Reply{}
	case "result":
		value, err = c.Execution.Result(ctx, in.(Target))
	case "events":
		if c.Events == nil {
			value = backend.Events{Unavailable: true, Gap: true}
		} else {
			value, err = c.Events.ReadEvents(ctx, in.(Poll))
		}
	case "interrupt":
		err = c.Execution.Interrupt(ctx, in.(Target))
		value = Reply{}
	case "steer":
		var yes bool
		yes, err = c.Execution.Steer(ctx, in.(Target))
		value = Reply{Value: yes}
	case "close":
		err = c.Execution.Close(ctx, in.(Target))
		value = Reply{}
	case "cleanup":
		err = c.Execution.Cleanup(ctx, in.(Scope))
		value = Reply{}
	case "state":
		value, err = c.Execution.Pending(ctx, in.(Scope))
	default:
		return fmt.Errorf("unsupported concrete task operation %s", op)
	}
	if err != nil {
		return err
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}
