package rootlogin

import "testing"

func TestRootLoginDisabled(t *testing.T) {
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
			input: "# PermitRootLogin no\n",
			want:  false,
		},
		{
			name:  "disabled",
			input: "PermitRootLogin no\n",
			want:  true,
		},
		{
			name:  "enabled",
			input: "PermitRootLogin yes\n",
			want:  false,
		},
		{
			name:  "mixed_case_value",
			input: "PermitRootLogin No\n",
			want:  true,
		},
		{
			name:  "inline_comment",
			input: "PermitRootLogin no # managed by settled\n",
			want:  true,
		},
		{
			name:  "leading_whitespace",
			input: "  \tPermitRootLogin\tno\n",
			want:  true,
		},
		{
			name:  "non_no_value",
			input: "PermitRootLogin prohibit-password\n",
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
