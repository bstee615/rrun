// Package sshconf reads ~/.ssh/config to resolve host aliases into connection
// details, used for display purposes in "rrun remote show".
// SSH itself already reads this config transparently, so rrun doesn't need to
// duplicate settings — it just surfaces them for the user.
package sshconf

import (
	"strconv"

	"github.com/kevinburke/ssh_config"
)

// Info contains resolved SSH connection details for a host alias.
type Info struct {
	Alias        string // as given in rrun config
	Hostname     string // resolved HostName (or same as Alias if not in ssh config)
	User         string // resolved User (empty if not set)
	Port         int    // resolved Port (0 = default 22)
	IdentityFile string // resolved IdentityFile (empty if not set)
}

// Resolve looks up a host alias in ~/.ssh/config and returns resolved details.
// If the alias isn't in SSH config (e.g. it's a bare IP), fields are left empty.
func Resolve(alias string) Info {
	info := Info{Alias: alias}

	info.Hostname = ssh_config.Get(alias, "HostName")
	if info.Hostname == "" {
		info.Hostname = alias
	}
	info.User = ssh_config.Get(alias, "User")
	if portStr := ssh_config.Get(alias, "Port"); portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil && p > 0 {
			info.Port = p
		}
	}
	info.IdentityFile = ssh_config.Get(alias, "IdentityFile")
	return info
}
