# Installation

This guide covers all installation methods for traefikctl.

## Requirements

- **OS**: Linux (Ubuntu, Debian, CentOS, Fedora, etc.)
- **Architecture**: amd64, arm64
- **Privileges**: sudo/root for system-wide installation
- **Traefik** (optional): Will be installed alongside if needed

## Install via Binary

### Automatic Installation

```bash
curl -fsSL https://raw.githubusercontent.com/eliasmeireles/traefikctl/main/install.sh | bash
```

### Manual Installation

1. Download the latest release for your architecture:

```bash
# amd64
curl -LO https://github.com/eliasmeireles/traefikctl/releases/latest/download/traefikctl-linux-amd64.tar.gz

# arm64
curl -LO https://github.com/eliasmeireles/traefikctl/releases/latest/download/traefikctl-linux-arm64.tar.gz
```

2. Extract and install:

```bash
tar -xzf traefikctl-*.tar.gz
sudo mv traefikctl /usr/local/bin/
sudo chmod +x /usr/local/bin/traefikctl
```

3. Verify installation:

```bash
traefikctl version
```

## Install via Go

If you have Go installed:

```bash
go install github.com/eliasmeireles/traefikctl@latest
```

This installs to `$GOPATH/bin/traefikctl`.

## Build from Source

```bash
git clone https://github.com/eliasmeireles/traefikctl.git
cd traefikctl
make build
sudo make install
```

## Install Traefik

After installing traefikctl, install Traefik:

```bash
# Install latest version
traefikctl install

# Install specific version
traefikctl install --version v3.4.0

# Check if already installed
traefikctl install --check
```

## Shell Completion

### Bash

```bash
# Add to ~/.bashrc
source <(traefikctl completion bash)

# Or install completion file
traefikctl completion bash > /etc/bash_completion.d/traefikctl
```

### Zsh

```bash
# Add to ~/.zshrc
source <(traefikctl completion zsh)

# Or install completion file
traefikctl completion zsh > "${fpath[1]}/_traefikctl"
```

### Fish

```bash
traefikctl completion fish > ~/.config/fish/completions/traefikctl.fish
```

## Post-Installation Steps

1. **Verify the installation**:

```bash
traefikctl check
```

2. **Generate initial configuration**:

```bash
traefikctl config --generate
```

3. **Configure for HTTPS** (optional):

```bash
traefikctl config --generate --acme --acme-email your@email.com
```

## Uninstallation

To remove traefikctl:

```bash
# Remove binary
sudo rm /usr/local/bin/traefikctl

# Remove configurations (optional)
sudo rm -rf /etc/traefikctl

# Remove Traefik (optional)
sudo rm /usr/local/bin/traefik
sudo rm -rf /etc/traefik
sudo rm -rf /var/log/traefik
```

## Troubleshooting

### Permission Denied

If you encounter permission errors:

```bash
sudo traefikctl <command>
```

Or add your user to the appropriate groups:

```bash
sudo usermod -aG systemd-journal $USER
```

### Command Not Found

Ensure `/usr/local/bin` is in your PATH:

```bash
export PATH="/usr/local/bin:$PATH"
```
