package fail2ban_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/tpodg/settled/internal/server"
	"github.com/tpodg/settled/internal/strutil"
	"github.com/tpodg/settled/internal/task"
	"github.com/tpodg/settled/internal/task/fail2ban"
	"github.com/tpodg/settled/internal/task/taskutil"
	"github.com/tpodg/settled/internal/testutils"
	tasktests "github.com/tpodg/settled/internal/testutils/task"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func TestFail2banTask_Integration(t *testing.T) {
	ctx := context.Background()
	password := "test-pass-123!"
	sshC := testutils.SetupSSHContainerWithOptions(t, ctx, testutils.SSHContainerOptions{
		UserPassword:   password,
		EnableNetAdmin: true,
	})
	defer sshC.Container.Terminate(ctx)

	time.Sleep(2 * time.Second)

	srv := server.NewSSHServer("fail2ban-integration", sshC.Address, server.User{
		Name:   "testuser",
		SSHKey: sshC.KeyPath,
	}, sshC.KnownHostsPath, server.SSHOptions{})

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelInfo}))
	runner := task.NewRunner(logger)

	prefix, err := taskutil.SudoPrefix(ctx, srv)
	if err != nil {
		t.Fatalf("resolve sudo prefix: %v", err)
	}

	logPath := "/var/log/auth.log"
	maxRetry := 6
	overrides := map[string]any{
		fail2ban.TaskKey: map[string]any{
			"rules": map[string]any{
				"sshd": map[string]any{
					"enabled":   true,
					"filter":    "sshd",
					"port":      "ssh",
					"logpath":   []string{logPath},
					"backend":   "polling",
					"max_retry": maxRetry,
					"find_time": "5m",
					"ban_time":  "5m",
					"options": map[string]any{
						"banaction": "iptables-multiport",
					},
				},
			},
		},
	}

	tasks := tasktests.PlanTasks(t, overrides, fail2ban.Spec())
	tasktests.AssertTasksNeedExecution(t, ctx, srv, tasks)
	if err := runner.Run(ctx, srv, tasks...); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	waitForJail(t, ctx, srv, prefix, "sshd")

	baselineLog := readAuthLogTail(t, ctx, srv, prefix, logPath)
	baselineCount := countAuthFailures(baselineLog, "testuser")

	for i := 0; i < 2; i++ {
		if err := attemptPasswordLogin(ctx, sshC.Address, "testuser", "wrong-pass", sshC.KnownHostsPath); err == nil {
			t.Fatal("expected invalid password attempt to fail")
		}
	}

	updatedLog := waitForAuthLogUpdate(t, ctx, srv, prefix, logPath, "testuser", baselineCount, 2)
	ip := lastAuthFailureIP(updatedLog, "testuser")
	if ip == "" {
		t.Fatal("expected auth log to contain failed password entries with IP")
	}
	baselineIPCount := countAuthFailuresForIP(baselineLog, "testuser", ip)
	updatedCount := countAuthFailuresForIP(updatedLog, "testuser", ip)
	if updatedCount-baselineIPCount < 2 {
		t.Fatalf("expected at least 2 new auth failures for %s, got %d", ip, updatedCount-baselineIPCount)
	}
	if updatedCount >= maxRetry {
		t.Fatalf("unexpected %d failures for %s before ban attempt", updatedCount, ip)
	}

	if err := attemptPasswordLogin(ctx, sshC.Address, "testuser", password, sshC.KnownHostsPath); err != nil {
		t.Fatalf("expected valid login to succeed before ban: %v", err)
	}

	remaining := maxRetry - updatedCount
	for i := 0; i < remaining; i++ {
		if err := attemptPasswordLogin(ctx, sshC.Address, "testuser", "wrong-pass", sshC.KnownHostsPath); err == nil {
			t.Fatal("expected invalid password attempt to fail")
		}
	}

	waitForLoginFailure(t, ctx, sshC.Address, "testuser", password, sshC.KnownHostsPath)
}

func attemptPasswordLogin(ctx context.Context, address, user, password, knownHostsPath string) error {
	timeout := 5 * time.Second
	loginCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	hostKeyCallback, err := knownhosts.New(knownHostsPath)
	if err != nil {
		return fmt.Errorf("load known_hosts: %w", err)
	}
	config := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: hostKeyCallback,
		Timeout:         timeout,
	}

	var dialer net.Dialer
	conn, err := dialer.DialContext(loginCtx, "tcp", address)
	if err != nil {
		return fmt.Errorf("dial ssh: %w", err)
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return fmt.Errorf("set handshake deadline: %w", err)
	}
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, address, config)
	if err != nil {
		return err
	}
	if err := conn.SetDeadline(time.Time{}); err != nil {
		return fmt.Errorf("clear deadline: %w", err)
	}
	client := ssh.NewClient(sshConn, chans, reqs)
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("new session: %w", err)
	}
	defer session.Close()

	if err := session.Run("true"); err != nil {
		return fmt.Errorf("run: %w", err)
	}
	return nil
}

func readAuthLogTail(t *testing.T, ctx context.Context, srv server.Server, prefix, path string) string {
	t.Helper()

	script := fmt.Sprintf("tail -n 200 %s", strutil.ShellEscape(path))
	cmd := prefix + "sh -c " + strutil.ShellEscape(script)
	output, err := srv.Execute(ctx, cmd)
	if err != nil {
		t.Fatalf("read auth log %s: %v", path, err)
	}
	return output
}

func countAuthFailuresForIP(output, user, ip string) int {
	var count int
	for _, line := range strings.Split(output, "\n") {
		if !strings.Contains(line, "Failed password for "+user) &&
			!strings.Contains(line, "Failed password for invalid user "+user) {
			continue
		}
		if extractIP(line) != ip {
			continue
		}
		count++
	}
	return count
}

func countAuthFailures(output, user string) int {
	var count int
	for _, line := range strings.Split(output, "\n") {
		if !strings.Contains(line, "Failed password for "+user) &&
			!strings.Contains(line, "Failed password for invalid user "+user) {
			continue
		}
		count++
	}
	return count
}

func lastAuthFailureIP(output, user string) string {
	var lastIP string
	for _, line := range strings.Split(output, "\n") {
		if !strings.Contains(line, "Failed password for "+user) &&
			!strings.Contains(line, "Failed password for invalid user "+user) {
			continue
		}
		ip := extractIP(line)
		if ip == "" {
			continue
		}
		lastIP = ip
	}
	return lastIP
}

func waitForAuthLogUpdate(t *testing.T, ctx context.Context, srv server.Server, prefix, path, user string, baseline, minDelta int) string {
	t.Helper()

	deadline := time.Now().Add(20 * time.Second)
	for {
		output := readAuthLogTail(t, ctx, srv, prefix, path)
		if countAuthFailures(output, user) >= baseline+minDelta {
			return output
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected %d new auth failures in %s", minDelta, path)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func extractIP(line string) string {
	fields := strings.Fields(line)
	for i, field := range fields {
		if field == "from" && i+1 < len(fields) {
			return fields[i+1]
		}
	}
	return ""
}

func waitForJail(t *testing.T, ctx context.Context, srv server.Server, prefix, jail string) {
	t.Helper()

	deadline := time.Now().Add(20 * time.Second)
	cmd := prefix + "fail2ban-client status"
	var lastOutput string
	for {
		output, err := srv.Execute(ctx, cmd)
		if err == nil && strings.Contains(output, "Jail list:") && strings.Contains(output, jail) {
			return
		}
		if err != nil {
			lastOutput = err.Error()
		} else {
			lastOutput = output
		}
		if time.Now().After(deadline) {
			t.Fatalf("fail2ban jail %q not ready, last output: %s", jail, strings.TrimSpace(lastOutput))
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func waitForLoginFailure(t *testing.T, ctx context.Context, address, user, password, knownHostsPath string) {
	t.Helper()

	deadline := time.Now().Add(30 * time.Second)
	for {
		err := attemptPasswordLogin(ctx, address, user, password, knownHostsPath)
		if err != nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("expected valid login to fail after ban")
		}
		time.Sleep(500 * time.Millisecond)
	}
}
