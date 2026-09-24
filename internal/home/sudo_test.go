package home

import (
	"errors"
	"os/user"
	"path/filepath"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func fixedUser(uid, gid, home string) func(string) (*user.User, error) {
	return func(string) (*user.User, error) {
		return &user.User{Uid: uid, Gid: gid, HomeDir: home}, nil
	}
}

func failingLookup(string) (*user.User, error) {
	return nil, errors.New("not found")
}

func TestSudoOwnerFromEnv(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		env        map[string]string
		lookupID   func(string) (*user.User, error)
		lookupName func(string) (*user.User, error)
		wantOwner  bool
		wantUID    int
		wantHome   string
	}{
		{
			name:      "SUDO_UID resolves the invoking user",
			env:       map[string]string{"SUDO_UID": "1000"},
			lookupID:  fixedUser("1000", "1000", "/home/dev"),
			wantOwner: true, wantUID: 1000, wantHome: "/home/dev",
		},
		{
			name:       "falls back to SUDO_USER when id lookup fails",
			env:        map[string]string{"SUDO_UID": "1000", "SUDO_USER": "dev"},
			lookupID:   failingLookup,
			lookupName: fixedUser("1001", "1001", "/home/dev"),
			wantOwner:  true, wantUID: 1001, wantHome: "/home/dev",
		},
		{
			name:      "root-owned sudo grants nothing",
			env:       map[string]string{"SUDO_UID": "0"},
			lookupID:  fixedUser("0", "0", "/root"),
			wantOwner: false,
		},
		{
			name:       "SUDO_USER=root grants nothing",
			env:        map[string]string{"SUDO_USER": "root"},
			lookupName: fixedUser("0", "0", "/root"),
			wantOwner:  false,
		},
		{
			name:      "no sudo env means no override",
			env:       map[string]string{},
			lookupID:  fixedUser("1000", "1000", "/home/dev"),
			wantOwner: false,
		},
		{
			name:      "user record without home is rejected",
			env:       map[string]string{"SUDO_UID": "1000"},
			lookupID:  fixedUser("1000", "1000", ""),
			wantOwner: false,
		},
		{
			name:       "user record without numeric ids is rejected",
			env:        map[string]string{"SUDO_USER": "dev"},
			lookupName: fixedUser("dev", "dev", "/home/dev"),
			wantOwner:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			getenv := func(key string) string { return tc.env[key] }
			owner := sudoOwnerFromEnv(getenv, tc.lookupID, tc.lookupName)
			if !tc.wantOwner {
				require.Nil(t, owner)
				return
			}
			require.NotNil(t, owner)
			require.Equal(t, tc.wantUID, owner.uid)
			require.Equal(t, tc.wantHome, owner.homeDir)
		})
	}
}

func TestConfigUsesSudoHome(t *testing.T) {
	restore := sudoOwner
	t.Cleanup(func() { sudoOwner = restore })

	t.Setenv("XDG_CONFIG_HOME", "")
	sudoOwner = &ownerIDs{uid: 1000, gid: 1000, homeDir: "/home/dev"}

	require.Equal(t, filepath.Join("/home/dev", ".config"), Config())

	uid, gid, ok := Owner()
	require.True(t, ok)
	require.Equal(t, 1000, uid)
	require.Equal(t, 1000, gid)

	sudoOwner = nil
	_, _, ok = Owner()
	require.False(t, ok)
}

func TestConfigKeepsExplicitXDGOverride(t *testing.T) {
	restore := sudoOwner
	t.Cleanup(func() { sudoOwner = restore })

	sudoOwner = &ownerIDs{uid: 1000, gid: 1000, homeDir: "/home/dev"}
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(string(filepath.Separator), "custom", "config"))

	require.Equal(t, filepath.Join(string(filepath.Separator), "custom", "config"), Config())
}
