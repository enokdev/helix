package config

import (
	"os"
	"strings"
)

const activeProfilesEnv = "HELIX_PROFILES_ACTIVE"

func (l *loader) resolveProfiles() []string {
	if l.profilesSet {
		return append([]string(nil), l.profiles...)
	}
	return profilesFromEnv(os.Getenv(activeProfilesEnv))
}

func profilesFromEnv(value string) []string {
	if value == "" {
		return nil
	}
	return normalizeProfiles(strings.Split(value, ","))
}

func normalizeProfiles(profiles []string) []string {
	normalized := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		profile = strings.TrimSpace(profile)
		if profile == "" {
			continue
		}
		normalized = append(normalized, profile)
	}
	return normalized
}
