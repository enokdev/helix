package config

// Option configures a Loader.
type Option func(*loader)

// WithConfigPaths overrides the directories searched for application.yaml.
func WithConfigPaths(paths ...string) Option {
	return func(l *loader) {
		l.configPaths = append([]string(nil), paths...)
	}
}

// WithDefaults configures fallback values applied before YAML and env values.
func WithDefaults(values map[string]any) Option {
	return func(l *loader) {
		l.defaults = deepCopySettings(values)
	}
}

// WithProfiles configures explicit profile YAML files to merge after application.yaml.
func WithProfiles(profiles ...string) Option {
	return func(l *loader) {
		normalized := normalizeProfiles(profiles)
		if len(normalized) > 0 {
			l.profiles = normalized
			l.profilesSet = true
		}
	}
}

// WithEnvPrefix configures an optional environment variable prefix.
func WithEnvPrefix(prefix string) Option {
	return func(l *loader) {
		l.envPrefix = prefix
	}
}

// WithAllowMissingConfig lets Load continue when application.yaml is absent.
// Profile files, defaults, and environment variables can still provide values.
func WithAllowMissingConfig() Option {
	return func(l *loader) {
		l.allowMissing = true
	}
}
