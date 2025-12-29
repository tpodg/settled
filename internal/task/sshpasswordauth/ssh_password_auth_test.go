package sshpasswordauth

import (
	"fmt"
	"testing"

	"github.com/tpodg/settled/internal/sshd"
)

func TestSSHPasswordAuthDisabled(t *testing.T) {
	settingLine := func(key, value string) string {
		return fmt.Sprintf("%s %s\n", key, value)
	}

	cases := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "empty",
			input: "",
			want:  false,
		},
		{
			name:  "commented_only",
			input: fmt.Sprintf("# %s %s\n# %s %s\n", sshd.KeyPasswordAuthentication, sshd.ValueNo, sshd.KeyKbdInteractiveAuth, sshd.ValueNo),
			want:  false,
		},
		{
			name:  "disabled",
			input: settingLine(sshd.KeyPasswordAuthentication, sshd.ValueNo) + settingLine(sshd.KeyKbdInteractiveAuth, sshd.ValueNo),
			want:  true,
		},
		{
			name:  "password_enabled",
			input: settingLine(sshd.KeyPasswordAuthentication, "yes") + settingLine(sshd.KeyKbdInteractiveAuth, sshd.ValueNo),
			want:  false,
		},
		{
			name:  "kbd_enabled",
			input: settingLine(sshd.KeyPasswordAuthentication, sshd.ValueNo) + settingLine(sshd.KeyKbdInteractiveAuth, "yes"),
			want:  false,
		},
		{
			name:  "mixed_case_values",
			input: settingLine(sshd.KeyPasswordAuthentication, "No") + settingLine(sshd.KeyKbdInteractiveAuth, "NO"),
			want:  true,
		},
		{
			name:  "inline_comment",
			input: fmt.Sprintf("%s %s # managed by settled\n%s %s\n", sshd.KeyPasswordAuthentication, sshd.ValueNo, sshd.KeyKbdInteractiveAuth, sshd.ValueNo),
			want:  true,
		},
		{
			name:  "challenge_response_fallback",
			input: settingLine(sshd.KeyPasswordAuthentication, sshd.ValueNo) + settingLine(sshd.KeyChallengeResponseAuth, sshd.ValueNo),
			want:  true,
		},
		{
			name:  "kbd_overrides_challenge",
			input: settingLine(sshd.KeyPasswordAuthentication, sshd.ValueNo) + settingLine(sshd.KeyChallengeResponseAuth, sshd.ValueNo) + settingLine(sshd.KeyKbdInteractiveAuth, "yes"),
			want:  false,
		},
		{
			name:  "challenge_enabled_with_kbd_disabled",
			input: settingLine(sshd.KeyPasswordAuthentication, sshd.ValueNo) + settingLine(sshd.KeyKbdInteractiveAuth, sshd.ValueNo) + settingLine(sshd.KeyChallengeResponseAuth, "yes"),
			want:  false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := sshPasswordAuthDisabled(tc.input)
			if err != nil {
				t.Fatalf("sshPasswordAuthDisabled failed: %v", err)
			}
			if got != tc.want {
				t.Fatalf("expected %v, got %v", tc.want, got)
			}
		})
	}
}

func TestRenderScript(t *testing.T) {
	task := &DisableSSHPasswordAuthTask{}
	script, err := task.renderScript()
	if err != nil {
		t.Fatalf("renderScript failed: %v", err)
	}
	if script == "" {
		t.Fatal("renderScript returned empty script")
	}
}
