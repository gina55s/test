// Package zinit exposes function to interat with zinit service life cyle management
package zinit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestParseList(t *testing.T) {
	s := `
ntp: Running
telnetd: Running
network-dhcp: Success
haveged: Success
debug-tty: Running
routing: Success
udevd: Running
dhcp_test: Running
udev-trigger: Success
sshd-setup: Success
local-modprobe: Success
networkd: Error(Exited(Pid(1592), 1))
sshd: Running`
	services, err := parseList(s)
	require.NoError(t, err)

	assert.Equal(t, map[string]ServiceState{
		"ntp":            {state: ServiceStateRunning},
		"telnetd":        {state: ServiceStateRunning},
		"network-dhcp":   {state: ServiceStateSuccess},
		"haveged":        {state: ServiceStateSuccess},
		"debug-tty":      {state: ServiceStateRunning},
		"routing":        {state: ServiceStateSuccess},
		"udevd":          {state: ServiceStateRunning},
		"dhcp_test":       {state: ServiceStateRunning},
		"udev-trigger":   {state: ServiceStateSuccess},
		"sshd-setup":     {state: ServiceStateSuccess},
		"local-modprobe": {state: ServiceStateSuccess},
		"networkd":       {state: ServiceStateError, reason: "exited(pid(1592), 1)"},
		"sshd":           {state: ServiceStateRunning},
	}, services)
}

func TestParseStatus(t *testing.T) {
	s := `
name: ntp
pid: 223
state: Running
target: Up
log: Ring
after:
  - network-dhcp: Success`
	status, err := parseStatus(s)
	require.NoError(t, err)

	assert.Equal(t, ServiceStatus{
		Name:   "ntp",
		Pid:    223,
		State:  ServiceState{state: ServiceStateRunning},
		Target: ServiceTargetUp,
	}, status)

	assert.False(t, status.State.Exited())

	s = `
name: ntp
pid: 223
state: Error(exit reason)
target: Up
log: Ring
after:
  - network-dhcp: Success`
	status, err = parseStatus(s)
	require.NoError(t, err)

	assert.Equal(t, ServiceStatus{
		Name:   "ntp",
		Pid:    223,
		State:  ServiceState{state: ServiceStateError, reason: "exit reason"},
		Target: ServiceTargetUp,
	}, status)

	assert.True(t, status.State.Exited())
	assert.True(t, status.State.Is(ServiceStateError))
}

func TestParseService(t *testing.T) {
	b := []byte(`
exec: /bin/true
test: test -e /bin/true
oneshot: false
log: ring
after:
 - one
 - two
`)
	var s InitService
	err := yaml.Unmarshal(b, &s)
	require.NoError(t, err)

	assert.Equal(t, InitService{
		Exec:    "/bin/true",
		Test:    "test -e /bin/true",
		Oneshot: false,
		Log:     RingLogType,
		After:   []string{"one", "two"},
	}, s)
}
