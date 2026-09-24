// Package home provides utilities for dealing with the user's home directory.
package home

import (
	"cmp"
	"log/slog"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

var (
	homedir, homedirErr = os.UserHomeDir()

	// sudoOwner is the identity of the user who invoked sudo, set only
	// when this process runs as root through sudo. Prime reads its global
	// settings from that user's home (so an already configured provider
	// and model setup keeps working under "sudo prime") and hands files
	// it creates back to that user via [Chown].
	sudoOwner *ownerIDs
)

// ownerIDs is a numeric uid/gid pair plus the home directory belonging
// to that account.
type ownerIDs struct {
	uid     int
	gid     int
	homeDir string
}

func init() {
	if homedirErr != nil {
		slog.Error("Failed to get user home directory", "error", homedirErr)
	}
	sudoOwner = resolveSudoOwner()
}

// resolveSudoOwner reports the invoking user when this process runs as
// root through sudo, and nil for every other invocation. Without the
// override, root sees an empty /root config with no providers, the coder
// agent fails to initialize, and "sudo prime" is unusable.
func resolveSudoOwner() *ownerIDs {
	// os.Geteuid always returns 0 on Windows, so the platform check has
	// to come first there.
	if runtime.GOOS == "windows" || os.Geteuid() != 0 {
		return nil
	}
	return sudoOwnerFromEnv(os.Getenv, user.LookupId, user.Lookup)
}

// sudoOwnerFromEnv resolves the invoking user from SUDO_UID (preferred,
// numeric) or SUDO_USER (login name). The lookup functions are injected
// so tests can cover the resolution without real sudo credentials.
func sudoOwnerFromEnv(getenv func(string) string, lookupID, lookupName func(string) (*user.User, error)) *ownerIDs {
	if rawUID := getenv("SUDO_UID"); rawUID != "" && rawUID != "0" {
		if u, err := lookupID(rawUID); err == nil {
			if owner := ownerFromUser(u); owner != nil {
				return owner
			}
		}
	}
	if name := getenv("SUDO_USER"); name != "" && name != "root" {
		if u, err := lookupName(name); err == nil {
			if owner := ownerFromUser(u); owner != nil {
				return owner
			}
		}
	}
	return nil
}

// ownerFromUser converts an os/user record into ownerIDs, returning nil
// when the record lacks a home directory or numeric ids.
func ownerFromUser(u *user.User) *ownerIDs {
	if u == nil || u.HomeDir == "" {
		return nil
	}
	uid, uidErr := strconv.Atoi(u.Uid)
	gid, gidErr := strconv.Atoi(u.Gid)
	if uidErr != nil || gidErr != nil {
		return nil
	}
	return &ownerIDs{uid: uid, gid: gid, homeDir: u.HomeDir}
}

// Dir returns the user home directory.
func Dir() string {
	return homedir
}

// Config returns the user config directory: $XDG_CONFIG_HOME when set,
// otherwise <home>/.config. When Prime runs as root through sudo the
// invoking user's home takes the place of root's, so settings that
// already exist keep being found; XDG_CONFIG_HOME from the current
// environment still wins because it is an explicit override.
func Config() string {
	return cmp.Or(
		os.Getenv("XDG_CONFIG_HOME"),
		filepath.Join(settingsHome(), ".config"),
	)
}

// settingsHome returns the home whose .config holds Prime's global
// settings: the invoking user's when running as root through sudo.
func settingsHome() string {
	if sudoOwner != nil {
		return sudoOwner.homeDir
	}
	return homedir
}

// Owner reports the uid and gid of the user whose home hosts Prime's
// global settings when that is not the current process owner, i.e. when
// running as root through sudo. ok is false for ordinary runs.
func Owner() (uid, gid int, ok bool) {
	if sudoOwner == nil {
		return 0, 0, false
	}
	return sudoOwner.uid, sudoOwner.gid, true
}

// Chown hands ownership of path to [Owner]. It is a no-op for ordinary
// runs and on Windows. Failures are logged, not returned: an ownership
// hiccup must not break the current run, it only degrades things for
// the unprivileged user afterwards.
func Chown(path string) {
	uid, gid, ok := Owner()
	if !ok {
		return
	}
	if err := chown(path, uid, gid); err != nil {
		slog.Debug("Failed to chown path", "path", path, "error", err)
	}
}

// ChownTree hands ownership of path and everything under it to
// [Owner]. Like [Chown] it is a no-op outside root-under-sudo runs.
func ChownTree(path string) {
	if _, _, ok := Owner(); !ok {
		return
	}
	err := filepath.WalkDir(path, func(p string, _ os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		Chown(p)
		return nil
	})
	if err != nil {
		slog.Debug("Failed to chown path tree", "path", path, "error", err)
	}
}

// Short replaces the actual home path from [Dir] with `~`.
func Short(p string) string {
	if homedir == "" || !strings.HasPrefix(p, homedir) {
		return p
	}
	if len(p) == len(homedir) {
		return "~"
	}
	if !os.IsPathSeparator(p[len(homedir)]) {
		return p
	}
	return filepath.Join("~", strings.TrimPrefix(p, homedir))
}

// Long replaces the `~` with actual home path from [Dir].
func Long(p string) string {
	if homedir == "" || !strings.HasPrefix(p, "~") {
		return p
	}
	if len(p) == 1 {
		return homedir
	}
	if !os.IsPathSeparator(p[1]) {
		return p
	}
	return strings.Replace(p, "~", homedir, 1)
}
