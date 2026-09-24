//go:build !windows

package home

import "os"

func chown(path string, uid, gid int) error {
	return os.Lchown(path, uid, gid)
}
