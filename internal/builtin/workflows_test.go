package builtin

import (
	"regexp"
	"strings"
	"testing"

	"github.com/tylergannon/gimbal/internal/modelalias"
)

func TestRoleDefaultsUseUnversionedAliases(t *testing.T) {
	aliasName := regexp.MustCompile(`^[a-z][a-z-]*$`)
	for role, spec := range Defaults() {
		name, effort, explicit := strings.Cut(spec, ":")
		resolved, err := modelalias.Resolve(modelalias.Selection{Name: name, Effort: effort, EffortPresent: explicit})
		if err != nil || !aliasName.MatchString(name) || resolved.Version == "" {
			t.Errorf("role %s must use an unversioned catalog alias: %q (%v)", role, spec, err)
		}
	}
}
