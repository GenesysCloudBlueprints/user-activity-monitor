package groupconfig

import "strings"

type TimeoutGroup struct {
	Name           string
	TimeoutMinutes int64
}

/**
 * Timeout Groups
 *
 * These are the groups that are used to determine the timeout for a user.
 *
 * The key is the Genesys group ID.
 *
 * The value is the name of the group (non-functional, for display purposes only) and the timeout in minutes.
 *
 * These should be updated to reflect the group configuration for your org.
 */

var TimeoutGroups = map[string]TimeoutGroup{
	"e613e69c-a2d4-40fc-aba5-a9a5eb43eeef": {
		Name:           "Timeout Group - agents",
		TimeoutMinutes: 15,
	},
	"f42fd8d0-3c9b-4db4-b389-c845fcef92c9": {
		Name:           "Timeout Group - supervisors",
		TimeoutMinutes: 60,
	},
}

// IsPresenceTTLExempt checks if the presence is exempt from the TTL (i.e. offline or ACD)
func IsPresenceTTLExempt(systemPresence string) bool {
	p := strings.ToLower(systemPresence)
	return p == "offline" || p == "idle" || p == "on_queue"
}
