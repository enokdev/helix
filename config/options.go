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
		l.profiles = append([]string(nil), profiles...)
	}
}

// WithEnvPrefix configures an optional environment variable prefix.
func WithEnvPrefix(prefix string) Option {
	return func(l *loader) {
		l.envPrefix = prefix
	}
}
