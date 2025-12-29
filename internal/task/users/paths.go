package users

import (
	"fmt"
	"io/fs"
	"path"

	"github.com/tpodg/settled/internal/task/taskutil"
)

const (
	SudoersDir                         = "/etc/sudoers.d"
	SudoersFilePrefix                  = "settled-"
	SSHDirName                         = ".ssh"
	AuthorizedKeysFileName             = "authorized_keys"
	SSHDirMode             fs.FileMode = 0o700
	AuthorizedKeysMode     fs.FileMode = 0o600
	SudoersFileMode        fs.FileMode = 0o440
)

func SudoersFilePath(name string) string {
	return path.Join(SudoersDir, SudoersFilePrefix+taskutil.SanitizeFilename(name, "user"))
}

func SSHDirPath(home string) string {
	return path.Join(home, SSHDirName)
}

func AuthorizedKeysPath(home string) string {
	return path.Join(SSHDirPath(home), AuthorizedKeysFileName)
}

func fileModeString(mode fs.FileMode) string {
	return fmt.Sprintf("%o", mode.Perm())
}
