package rootlogin

import (
	"fmt"
	"testing"

	"github.com/tpodg/settled/internal/sshd"
)

func TestRootLoginDisabled(t *testing.T) {
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
			input: fmt.Sprintf("# %s %s\n", sshd.KeyPermitRootLogin, sshd.ValueNo),
			want:  false,
		},
		{
			name:  "disabled",
			input: settingLine(sshd.KeyPermitRootLogin, sshd.ValueNo),
			want:  true,
		},
		{
			name:  "enabled",
			input: settingLine(sshd.KeyPermitRootLogin, "yes"),
			want:  false,
		},
		{
			name:  "mixed_case_value",
			input: settingLine(sshd.KeyPermitRootLogin, "No"),
			want:  true,
		},
		{
			name:  "inline_comment",
			input: fmt.Sprintf("%s %s # managed by settled\n", sshd.KeyPermitRootLogin, sshd.ValueNo),
			want:  true,
		},
		{
			name:  "leading_whitespace",
			input: fmt.Sprintf("  \t%s\t%s\n", sshd.KeyPermitRootLogin, sshd.ValueNo),
			want:  true,
		},
		{
			name:  "non_no_value",
			input: settingLine(sshd.KeyPermitRootLogin, "prohibit-password"),
			want:  false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := rootLoginDisabled(tc.input)
			if err != nil {
				t.Fatalf("rootLoginDisabled failed: %v", err)
			}
			if got != tc.want {
				t.Fatalf("expected %v, got %v", tc.want, got)
			}
		})
	}
}

func TestRenderScript(t *testing.T) {
	task := &DisableRootLoginTask{}
	script, err := task.renderScript()
	if err != nil {
		t.Fatalf("renderScript failed: %v", err)
	}
	if script == "" {
		t.Fatal("renderScript returned empty script")
	}
}
