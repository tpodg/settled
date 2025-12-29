package sshpasswordauth

import "testing"

func TestSSHPasswordAuthDisabled(t *testing.T) {
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
			input: "# PasswordAuthentication no\n# KbdInteractiveAuthentication no\n",
			want:  false,
		},
		{
			name:  "disabled",
			input: "PasswordAuthentication no\nKbdInteractiveAuthentication no\n",
			want:  true,
		},
		{
			name:  "password_enabled",
			input: "PasswordAuthentication yes\nKbdInteractiveAuthentication no\n",
			want:  false,
		},
		{
			name:  "kbd_enabled",
			input: "PasswordAuthentication no\nKbdInteractiveAuthentication yes\n",
			want:  false,
		},
		{
			name:  "mixed_case_values",
			input: "PasswordAuthentication No\nKbdInteractiveAuthentication NO\n",
			want:  true,
		},
		{
			name:  "inline_comment",
			input: "PasswordAuthentication no # managed by settled\nKbdInteractiveAuthentication no\n",
			want:  true,
		},
		{
			name:  "challenge_response_fallback",
			input: "PasswordAuthentication no\nChallengeResponseAuthentication no\n",
			want:  true,
		},
		{
			name:  "kbd_overrides_challenge",
			input: "PasswordAuthentication no\nChallengeResponseAuthentication no\nKbdInteractiveAuthentication yes\n",
			want:  false,
		},
		{
			name:  "challenge_enabled_with_kbd_disabled",
			input: "PasswordAuthentication no\nKbdInteractiveAuthentication no\nChallengeResponseAuthentication yes\n",
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
