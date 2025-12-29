#!/bin/sh
set -e

USER_NAME="${USER_NAME:-testuser}"
PUBLIC_KEY="${PUBLIC_KEY:-}"
USER_PASSWORD="${USER_PASSWORD:-}"
SUDO_NOPASSWD="${SUDO_NOPASSWD:-1}"

if ! id "$USER_NAME" >/dev/null 2>&1; then
  useradd --create-home --shell /bin/bash "$USER_NAME"
fi
if [ -n "$USER_PASSWORD" ]; then
  printf '%s:%s\n' "$USER_NAME" "$USER_PASSWORD" | chpasswd
fi

if [ -n "$PUBLIC_KEY" ]; then
  SSH_DIR="/home/$USER_NAME/.ssh"
  mkdir -p "$SSH_DIR"
  chmod 700 "$SSH_DIR"
  printf '%s\n' "$PUBLIC_KEY" > "$SSH_DIR/authorized_keys"
  chmod 600 "$SSH_DIR/authorized_keys"
  chown -R "$USER_NAME:$USER_NAME" "$SSH_DIR"

  ROOT_SSH_DIR="/root/.ssh"
  mkdir -p "$ROOT_SSH_DIR"
  chmod 700 "$ROOT_SSH_DIR"
  printf '%s\n' "$PUBLIC_KEY" > "$ROOT_SSH_DIR/authorized_keys"
  chmod 600 "$ROOT_SSH_DIR/authorized_keys"
fi

if command -v sudo >/dev/null 2>&1; then
  usermod -aG sudo "$USER_NAME" || true
  mkdir -p /etc/sudoers.d
  case "$SUDO_NOPASSWD" in
    0|false|FALSE|no|NO)
      printf '%s ALL=(ALL) ALL\n' "$USER_NAME" > /etc/sudoers.d/99-settled-test
      ;;
    *)
      printf '%s ALL=(ALL) NOPASSWD:ALL\n' "$USER_NAME" > /etc/sudoers.d/99-settled-test
      ;;
  esac
  chmod 0440 /etc/sudoers.d/99-settled-test
fi

if [ -f /etc/ssh/sshd_config ]; then
  printf '\nPubkeyAuthentication yes\nPasswordAuthentication yes\nPermitRootLogin yes\n' >> /etc/ssh/sshd_config
fi

ssh-keygen -A

exec /usr/sbin/sshd -D -e
