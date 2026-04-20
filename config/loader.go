package config

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/spf13/viper"
)

const (
	defaultConfigName = "application"
	defaultConfigType = "yaml"
	defaultConfigPath = "config"
)

// Loader loads application configuration and exposes resolved values by key.
type Loader interface {
	Load(target any) error
	Lookup(key string) (any, bool)
	ConfigFileUsed() string
	AllSettings() map[string]any
	ActiveProfiles() []string
}

type loader struct {
	mu             sync.RWMutex
	viper          *viper.Viper
	configPaths    []string
	defaults       map[string]any
	profiles       []string
	profilesSet    bool
	allowMissing   bool
	activeProfiles []string
	envPrefix      string
	configFileUsed string
	loaded         bool
}

type loadedConfig struct {
	viper          *viper.Viper
	configFileUsed string
	activeProfiles []string
}

// NewLoader creates a Viper-backed configuration loader.
func NewLoader(opts ...Option) Loader {
	l := &loader{
		configPaths: []string{defaultConfigPath},
		defaults:    make(map[string]any),
	}
	for _, opt := range opts {
		opt(l)
	}
	l.viper = l.newViper(defaultConfigName)
	return l
}

// Load reads configuration sources and decodes them into target.
// Load is safe to call from multiple goroutines but serialises writers.
func (l *loader) Load(target any) error {
	if !isValidDecodeTarget(target) {
		return fmt.Errorf("config: decode target %T: %w", target, ErrInvalidConfig)
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	next, err := l.readIntoNewViper()
	if err != nil {
		return err
	}
	if err := decodeInto(next.viper, target); err != nil {
		return err
	}
	l.applyLoadedConfig(next)
	return nil
}

// Lookup returns the resolved value for key after configuration has been loaded.
func (l *loader) Lookup(key string) (any, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if l.viper == nil || !l.loaded || !l.viper.IsSet(key) {
		return nil, false
	}
	return l.viper.Get(key), true
}

// ConfigFileUsed returns the base application.yaml path loaded by this loader.
func (l *loader) ConfigFileUsed() string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.configFileUsed
}

// AllSettings returns a deep copy of the resolved settings map.
func (l *loader) AllSettings() map[string]any {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if l.viper == nil || !l.loaded {
		return map[string]any{}
	}
	return deepCopySettings(l.viper.AllSettings())
}

// ActiveProfiles returns the profiles resolved during the last successful load.
func (l *loader) ActiveProfiles() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if !l.loaded {
		return []string{}
	}
	return append([]string(nil), l.activeProfiles...)
}

// readIntoNewViper builds a fresh viper instance and loads all config sources.
// Must be called with l.mu held for writing.
func (l *loader) readIntoNewViper() (loadedConfig, error) {
	next := l.newViper(defaultConfigName)
	for key, value := range l.defaults {
		next.SetDefault(key, value)
	}

	configFileUsed := ""
	if err := next.ReadInConfig(); err != nil {
		if !l.allowMissing {
			return loadedConfig{}, wrapReadError("read application.yaml", err)
		}
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			return loadedConfig{}, wrapReadError("read application.yaml", err)
		}
	} else {
		configFileUsed = next.ConfigFileUsed()
	}

	activeProfiles := l.resolveProfiles()
	for _, profile := range activeProfiles {
		if err := l.mergeProfileInto(next, profile); err != nil {
			return loadedConfig{}, err
		}
	}

	// Bind all known keys so that ENV overrides are visible during Unmarshal.
	// Covers both YAML-sourced keys and defaults-only keys that have no YAML entry.
	allKeys := make(map[string]struct{})
	for _, key := range next.AllKeys() {
		allKeys[key] = struct{}{}
	}
	for key := range l.defaults {
		allKeys[key] = struct{}{}
	}
	for _, key := range knownConfigKeys {
		allKeys[key] = struct{}{}
	}
	for key := range allKeys {
		if err := next.BindEnv(key); err != nil {
			return loadedConfig{}, fmt.Errorf("config: bind env %q: %w", key, ErrInvalidConfig)
		}
	}

	return loadedConfig{
		viper:          next,
		configFileUsed: configFileUsed,
		activeProfiles: append([]string(nil), activeProfiles...),
	}, nil
}

func (l *loader) applyLoadedConfig(next loadedConfig) {
	l.viper = next.viper
	l.configFileUsed = next.configFileUsed
	l.activeProfiles = append([]string(nil), next.activeProfiles...)
	l.loaded = true
}

// mergeProfileInto merges the named profile YAML on top of the provided base config.
// A missing profile file is silently skipped — profiles are optional.
func (l *loader) mergeProfileInto(base *viper.Viper, profile string) error {
	profileViper := l.newProfileViper(defaultConfigName + "-" + profile)
	if err := profileViper.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if errors.As(err, &notFound) {
			return nil
		}
		return fmt.Errorf("config: read application-%s.yaml: %w", profile, ErrInvalidConfig)
	}
	if err := base.MergeConfigMap(profileViper.AllSettings()); err != nil {
		return fmt.Errorf("config: merge profile %q: %w", profile, ErrInvalidConfig)
	}
	return nil
}

// newViper returns a viper instance configured for the main application config,
// with AutomaticEnv enabled so ENV variables participate in Get/IsSet resolution.
func (l *loader) newViper(configName string) *viper.Viper {
	v := viper.New()
	v.SetConfigName(configName)
	v.SetConfigType(defaultConfigType)
	for _, path := range l.configPaths {
		v.AddConfigPath(path)
	}
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	if l.envPrefix != "" {
		v.SetEnvPrefix(l.envPrefix)
	}
	v.AutomaticEnv()
	return v
}

// newProfileViper returns a viper instance for reading a profile YAML file only.
// AutomaticEnv is intentionally disabled so AllSettings() returns raw YAML values
// without ENV contamination — ENV precedence is enforced by the base viper instance.
func (l *loader) newProfileViper(configName string) *viper.Viper {
	v := viper.New()
	v.SetConfigName(configName)
	v.SetConfigType(defaultConfigType)
	for _, path := range l.configPaths {
		v.AddConfigPath(path)
	}
	return v
}

func wrapReadError(action string, err error) error {
	var notFound viper.ConfigFileNotFoundError
	if errors.As(err, &notFound) {
		return fmt.Errorf("config: %s: %w", action, ErrConfigNotFound)
	}
	return fmt.Errorf("config: %s: %w", action, ErrInvalidConfig)
}

func isValidDecodeTarget(target any) bool {
	value := reflect.ValueOf(target)
	if !value.IsValid() || value.Kind() != reflect.Ptr || value.IsNil() {
		return false
	}
	return value.Elem().CanSet()
}

func decodeInto(v *viper.Viper, target any) error {
	targetValue := reflect.ValueOf(target)
	nextTarget := reflect.New(targetValue.Elem().Type())
	if err := v.Unmarshal(nextTarget.Interface()); err != nil {
		return fmt.Errorf("config: decode target %T: %w", target, ErrInvalidConfig)
	}
	targetValue.Elem().Set(nextTarget.Elem())
	return nil
}

// deepCopySettings returns a recursive copy of a settings map so that mutations
// by the caller cannot affect the loader's internal viper state.
func deepCopySettings(values map[string]any) map[string]any {
	copied := make(map[string]any, len(values))
	for key, value := range values {
		if nested, ok := value.(map[string]any); ok {
			copied[key] = deepCopySettings(nested)
		} else {
			copied[key] = value
		}
	}
	return copied
}
