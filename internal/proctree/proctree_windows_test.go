//go:build windows

package proctree

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
	"github.com/SpherePrime/CLI/vendordeps/x/sys/windows"
)

func TestProcTreeHelper(t *testing.T) {
	switch os.Getenv("PROCTREE_HELPER") {
	case "parent":
		child := exec.Command(os.Args[0], "-test.run=TestProcTreeHelper")
		child.Env = append(os.Environ(), "PROCTREE_HELPER=sleep")
		if err := child.Start(); err != nil {
			os.Exit(2)
		}
		fmt.Println(child.Process.Pid)
		time.Sleep(time.Minute)
		os.Exit(0)
	case "sleep":
		time.Sleep(time.Minute)
		os.Exit(0)
	}
}

func startTree(t *testing.T) (*exec.Cmd, *Group, int) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=TestProcTreeHelper")
	cmd.Env = append(os.Environ(), "PROCTREE_HELPER=parent")
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	Prepare(cmd)
	require.NoError(t, cmd.Start())
	group, err := Attach(cmd)
	require.NoError(t, err)
	line, err := bufio.NewReader(stdout).ReadString('\n')
	require.NoError(t, err)
	grandchild, err := strconv.Atoi(strings.TrimSpace(line))
	require.NoError(t, err)
	t.Cleanup(func() {
		group.Kill()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})
	return cmd, group, grandchild
}

func exited(pid int, timeout time.Duration) bool {
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return true
	}
	defer windows.CloseHandle(handle)
	event, _ := windows.WaitForSingleObject(handle, uint32(timeout.Milliseconds()))
	return event == windows.WAIT_OBJECT_0
}

func TestKillReapsGrandchildAfterDirectChildIsKilled(t *testing.T) {
	cmd, group, grandchild := startTree(t)
	require.NoError(t, cmd.Process.Kill())
	_ = cmd.Wait()
	require.False(t, exited(grandchild, 500*time.Millisecond), "grandchild must outlive its parent without the job")
	group.Kill()
	require.True(t, exited(grandchild, 5*time.Second), "job kill must reap the grandchild")
}

func TestKillReapsWholeTree(t *testing.T) {
	cmd, group, grandchild := startTree(t)
	group.Kill()
	require.True(t, exited(grandchild, 5*time.Second))
	require.True(t, exited(cmd.Process.Pid, 5*time.Second))
	group.Kill()
}
