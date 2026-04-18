package testutil

import (
	"github.com/enokdev/helix/config"
	"github.com/enokdev/helix/core"
)

// Option configures a Helix test application.
type Option func(*appOptions)

type appOptions struct {
	components       []any
	configPaths      []string
	configDefaults   map[string]any
	containerOptions []core.Option
}

func defaultAppOptions() appOptions {
	return appOptions{
		configPaths: []string{"config"},
	}
}

// WithComponents registers components in the test container before startup.
func WithComponents(components ...any) Option {
	return func(opts *appOptions) {
		opts.components = append(opts.components, components...)
	}
}

// WithConfigPaths overrides directories searched for application test config.
func WithConfigPaths(paths ...string) Option {
	return func(opts *appOptions) {
		opts.configPaths = append([]string(nil), paths...)
	}
}

// WithConfigDefaults configures fallback values for the test config loader.
func WithConfigDefaults(values map[string]any) Option {
	return func(opts *appOptions) {
		opts.configDefaults = copySettings(values)
	}
}

// WithContainerOptions appends core container options to the test container.
func WithContainerOptions(containerOptions ...core.Option) Option {
	return func(opts *appOptions) {
		opts.containerOptions = append(opts.containerOptions, containerOptions...)
	}
}

func (opts appOptions) configLoaderOptions() []config.Option {
	loaderOptions := []config.Option{
		config.WithAllowMissingConfig(),
		config.WithConfigPaths(opts.configPaths...),
		config.WithProfiles("test"),
	}
	if opts.configDefaults != nil {
		loaderOptions = append(loaderOptions, config.WithDefaults(opts.configDefaults))
	}
	return loaderOptions
}

func copySettings(values map[string]any) map[string]any {
	if values == nil {
		return nil
	}
	copied := make(map[string]any, len(values))
	for key, value := range values {
		if nested, ok := value.(map[string]any); ok {
			copied[key] = copySettings(nested)
		} else {
			copied[key] = value
		}
	}
	return copied
}
