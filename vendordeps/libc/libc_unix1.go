// Copyright 2024 The Libc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unix && !illumos && !(linux && (amd64 || arm64 || loong64 || ppc64le || s390x || riscv64 || 386 || arm)) && !openbsd

package libc // import "github.com/SpherePrime/CLI/vendordeps/libc"

import (
	"github.com/SpherePrime/CLI/vendordeps/x/sys/unix"

	"github.com/SpherePrime/CLI/vendordeps/libc/sys/types"
)

// ssize_t recvmsg(int sockfd, struct msghdr *msg, int flags);
func Xrecvmsg(t *TLS, sockfd int32, msg uintptr, flags int32) types.Ssize_t {
	if __ccgo_strace {
		trc("t=%v sockfd=%v msg=%v flags=%v, (%v:)", t, sockfd, msg, flags, origin(2))
	}
	n, _, err := unix.Syscall(unix.SYS_RECVMSG, uintptr(sockfd), msg, uintptr(flags))
	if err != 0 {
		t.setErrno(err)
		return -1
	}

	return types.Ssize_t(n)
}
