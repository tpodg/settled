package users

import (
	"testing"
)

func TestUserRenderScript(t *testing.T) {
	cases := []struct {
		name string
		task *UserTask
	}{
		{
			name: "full",
			task: &UserTask{
				name: "alice",
				config: UserConfig{
					Sudo:           true,
					SudoNoPassword: true,
					Groups:         []string{"sudo"},
					AuthorizedKeys: []string{"ssh-ed25519 AAAAC3..."},
				},
			},
		},
		{
			name: "minimal",
			task: &UserTask{
				name: "bob",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			script, err := tc.task.renderScript()
			if err != nil {
				t.Fatalf("renderScript failed: %v", err)
			}
			if script == "" {
				t.Fatal("renderScript returned empty script")
			}
		})
	}
}
