package observability

import (
	"context"
	"strings"

	"github.com/enokdev/helix/config"
)

const defaultVersion = "dev"

// InfoResponse is the JSON response returned by /actuator/info.
type InfoResponse struct {
	Version  string            `json:"version"`
	Profiles []string          `json:"profiles"`
	Build    map[string]string `json:"build"`
}

// InfoProvider supplies application metadata for /actuator/info.
type InfoProvider interface {
	Info(context.Context) InfoResponse
}

type defaultInfoProvider struct {
	loader  config.Loader
	version string
	build   map[string]string
}

// InfoOption configures the default info provider.
type InfoOption func(*defaultInfoProvider)

// WithVersion configures the version reported by /actuator/info.
func WithVersion(version string) InfoOption {
	return func(provider *defaultInfoProvider) {
		version = strings.TrimSpace(version)
		if version != "" {
			provider.version = version
		}
	}
}

// WithBuildInfo configures build metadata reported by /actuator/info.
func WithBuildInfo(build map[string]string) InfoOption {
	return func(provider *defaultInfoProvider) {
		provider.build = copyStringMap(build)
	}
}

// NewInfoProvider creates the default InfoProvider.
func NewInfoProvider(loader config.Loader, opts ...InfoOption) InfoProvider {
	provider := &defaultInfoProvider{
		loader:  loader,
		version: defaultVersion,
		build:   map[string]string{},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(provider)
		}
	}
	if provider.build == nil {
		provider.build = map[string]string{}
	}
	return provider
}

// Info returns stable application metadata.
func (p *defaultInfoProvider) Info(context.Context) InfoResponse {
	profiles := []string{}
	if p.loader != nil {
		profiles = p.loader.ActiveProfiles()
		if profiles == nil {
			profiles = []string{}
		}
	}

	return InfoResponse{
		Version:  p.version,
		Profiles: append([]string{}, profiles...),
		Build:    copyStringMap(p.build),
	}
}

func copyStringMap(input map[string]string) map[string]string {
	copied := make(map[string]string, len(input))
	for key, value := range input {
		copied[key] = value
	}
	return copied
}
