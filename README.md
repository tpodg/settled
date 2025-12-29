# Settled

Settled is a CLI tool designed to automate server configuration and hardening. It provides a robust foundation for preparing servers for production environments by streamlining common setup tasks.

## Features

- **Configuration-Driven**: Manage your server infrastructure through simple YAML configuration.
- **SSH-Based Execution**: Connects to remote servers using standard SSH protocols.
- **Multi-Server Support**: Easily target multiple servers in a single run.
- **Extensible**: Built with a modular architecture to support future provisioning and configuration providers.

### Feature Status

Status: ✅ available, 🚧 planned

- ✅ Create users
- ✅ Disable root SSH login
- ✅ Disable SSH password authentication
- 🚧 Install and configure Fail2ban
- 🚧 Install and configure firewall
- ...

## Getting Started

### Installation

To build the binary from source:

```bash
go build -o settle ./cmd/settle
```

### Configuration

Settled looks for a configuration file named `.settled.yaml` in the current directory or your home directory. You can use the provided example as a starting point:

```bash
cp .settled.yaml.example .settled.yaml
```

Edit `.settled.yaml` to define your servers and their connection details.

Settled verifies SSH host keys against a `known_hosts` file. By default it uses `~/.ssh/known_hosts`, or you can set `known_hosts` per server:

```yaml
servers:
  - name: web-1
    address: 1.2.3.4
    user:
      name: privileged_user
      ssh_key: ~/.ssh/id_ed25519
      # sudo_password: "optional sudo password"
    known_hosts: ~/.ssh/known_hosts
    use_agent: true
    handshake_timeout: 15s
```

`use_agent` controls whether the SSH agent is consulted (default true). `handshake_timeout` bounds the SSH handshake; it accepts duration strings like `10s` or `1m`. `sudo_password` is only used to elevate to sudo; SSH authentication still uses keys/agent.

### Tasks and Defaults

Tasks run with built-in defaults even if you provide no task configuration. You can override task settings per server:

```yaml
servers:
  - name: web-1
    address: 1.2.3.4
    user:
      name: privileged_user
      ssh_key: ~/.ssh/id_rsa
    tasks: {}
```
Task defaults live in `internal/task/defaults` as YAML.

### Bootstrapping the Initial Sudo User

Use the `bootstrap` command to create your first sudo user using a privileged login (defaults to root). This command runs only the bootstrap task and does not execute the normal `configure` task set. It uses the configured server list, but does not require any task configuration in YAML.

```bash
./settle bootstrap --user deploy
```

By default, it copies the login user's `authorized_keys` to the new user. To provide keys explicitly:

```bash
./settle bootstrap --user deploy --authorized-key "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA..."
```

Optional flags include `--login-user` (default `root`), `--group` (default `sudo`), `--sudo-nopasswd`, and `--sudo-password`. When `--login-user` is not root, it must have sudo privileges (use `--sudo-password` if it requires a password).

Example with a non-root login:

```bash
./settle bootstrap --user deploy --login-user admin
```

## Usage

Use the `--help` flag to see all available commands and options:

```bash
./settle --help
```

To see details for a specific command:

```bash
./settle [command] --help
```

## Tested Distributions

Settled is currently tested on:

- Ubuntu 24.04 (Noble Numbat)

## Testing

Integration tests require **Docker** to be running, as they use [Testcontainers](https://testcontainers.com/) to spin up real SSH server environments for verification.

To run all tests:

```bash
go test ./...
```
