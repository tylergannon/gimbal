package workflow

// Roles returns the roles the workflow names, in the order its source first
// creates a session for each: every NewSession anywhere in the body, and no
// fork, since a fork runs on its parent's binding. A run binds each of them.
func (g Graph) Roles() []string {
	var roles []string
	seen := map[string]bool{}
	var walk func(ops []Operation)
	walk = func(ops []Operation) {
		for _, op := range ops {
			switch op := op.(type) {
			case Session:
				if op.From == "" && !seen[op.Name] {
					seen[op.Name] = true
					roles = append(roles, op.Name)
				}
			case Scope:
				walk(op.Body)
			case PromiseLoop:
				walk(op.Body)
			case Iterate:
				walk(op.Body)
			case Repeat:
				walk(op.Body)
			case Group:
				for _, child := range op.Children {
					walk(child.Body)
				}
			case Condition:
				for _, branch := range op.Branches {
					walk(branch.Body)
				}
			}
		}
	}
	walk(g.Body)
	return roles
}
