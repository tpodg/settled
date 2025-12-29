package server

// User holds SSH credentials and optional sudo password for command execution.
type User struct {
	Name         string
	SSHKey       string
	SudoPassword string
}
