package utils

import (
	"strings"
)

// TeleportPrefix is the canonical prefix that all Giant Swarm Teleport kubeconfig
// contexts start with.
const TeleportPrefix = "teleport.giantswarm.io-"

// HasTeleportPrefix checks whether a context already begins with the canonical
// Teleport prefix.
func HasTeleportPrefix(ctx string) bool {
	return strings.HasPrefix(ctx, TeleportPrefix)
}

// StripTeleportPrefix removes the Teleport prefix from a context name if it is
// present. If not, it returns the original string.
func StripTeleportPrefix(ctx string) string {
	if HasTeleportPrefix(ctx) {
		return strings.TrimPrefix(ctx, TeleportPrefix)
	}
	return ctx
}

// BuildMcContext returns the full kubeconfig context name for a Management
// Cluster given its short name (e.g. "ghost") ->
// "teleport.giantswarm.io-ghost".
func BuildMcContext(mc string) string {
	if mc == "" {
		return ""
	}
	return TeleportPrefix + mc
}

// BuildWcContext returns the full kubeconfig context name for a Workload
// Cluster given the MC short name and WC short name. Example:
// "teleport.giantswarm.io-ghost-acme".
func BuildWcContext(mc, wc string) string {
	if mc == "" || wc == "" {
		return ""
	}
	return TeleportPrefix + mc + "-" + wc
}
