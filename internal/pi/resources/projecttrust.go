package resources

import (
	"context"

	"github.com/tylergannon/gimbal/internal/pi/config"
)

// ProjectTrustContext is the host surface used to ask the user about project
// trust. HasUI false means there is no way to ask and the headless answer is
// used.
type ProjectTrustContext struct {
	HasUI  bool
	Select func(ctx context.Context, message string, options []string) (string, error)
}

// ResolveProjectTrustedOptions configures ResolveProjectTrusted.
type ResolveProjectTrustedOptions struct {
	Cwd                 string
	TrustStore          *ProjectTrustStore
	TrustOverride       *bool
	DefaultProjectTrust config.DefaultProjectTrust
	ProjectTrustContext ProjectTrustContext
}

func formatProjectTrustPrompt(cwd string) string {
	return "Trust project folder?\n" + cwd + "\n\nThis allows pi to load .pi settings and resources, install missing project packages, and execute project extensions."
}

func selectProjectTrustOption(ctx context.Context, cwd string, trustContext ProjectTrustContext) (ProjectTrustOption, bool, error) {
	options := GetProjectTrustOptions(cwd, true)
	if trustContext.Select == nil {
		return ProjectTrustOption{}, false, nil
	}
	labels := make([]string, len(options))
	for i, option := range options {
		labels[i] = option.Label
	}
	selected, err := trustContext.Select(ctx, formatProjectTrustPrompt(cwd), labels)
	if err != nil {
		return ProjectTrustOption{}, false, err
	}
	for _, option := range options {
		if option.Label == selected {
			return option, true, nil
		}
	}
	return ProjectTrustOption{}, false, nil
}

// ResolveProjectTrusted decides whether project resources are trusted. It
// honors an explicit override, then the trust store, then the configured
// default, then asks when a UI is available. It returns false without a UI.
func ResolveProjectTrusted(ctx context.Context, options ResolveProjectTrustedOptions) (bool, error) {
	if options.TrustOverride != nil {
		return *options.TrustOverride, nil
	}
	if !HasTrustRequiringProjectResources(options.Cwd) {
		return true, nil
	}

	if options.TrustStore != nil {
		if decision := options.TrustStore.Get(options.Cwd); decision != nil {
			return *decision, nil
		}
	}

	switch options.DefaultProjectTrust {
	case config.ProjectTrustAlways:
		return true, nil
	case config.ProjectTrustNever:
		return false, nil
	}

	if !options.ProjectTrustContext.HasUI {
		return false, nil
	}

	selected, ok, err := selectProjectTrustOption(ctx, options.Cwd, options.ProjectTrustContext)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	if options.TrustStore != nil && len(selected.Updates) > 0 {
		options.TrustStore.SetMany(selected.Updates)
	}
	return selected.Trusted, nil
}
