# Delta for installer

## ADDED Requirements

### REQ-10: Default installation directory
The install script MUST default to `~/.local/bin` instead of `/usr/local/bin` when no installation directory is specified.

#### Scenario: Default install to user-local bin
- GIVEN user has no `--dir` or `--system` flags
- WHEN user runs `./install.sh`
- THEN script installs to `~/.local/bin/glpictl-ai`
- AND no sudo is required

### REQ-11: Directory creation
The install script MUST create the installation directory if it does not exist, without requiring sudo.

#### Scenario: Create missing directory
- GIVEN installation directory `~/.local/bin` does not exist
- WHEN script attempts installation
- THEN directory is created using `mkdir -p`
- AND installation proceeds without sudo

#### Scenario: Custom directory creation
- GIVEN user specifies `--dir /custom/path`
- AND `/custom/path` does not exist
- WHEN script attempts installation
- THEN `/custom/path` is created
- AND installation proceeds

### REQ-12: Custom directory flag
The install script MUST accept a `--dir <path>` flag to specify a custom installation directory.

#### Scenario: Install to custom directory
- GIVEN user runs `./install.sh --dir /opt/glpictl-ai`
- AND `/opt/glpictl-ai` exists and is writable
- THEN binary is installed to `/opt/glpictl-ai/glpictl-ai`
- AND PATH warning is shown if not in PATH

#### Scenario: Custom directory requires sudo
- GIVEN user specifies `--dir /usr/local/bin`
- AND user does not have write permissions
- WHEN script attempts installation
- THEN script fails with clear error message
- AND suggests using `--system` flag with sudo

### REQ-13: PATH warning
The install script MUST check if the installation directory is in the user's PATH and display a warning if it is not.

#### Scenario: Directory not in PATH
- GIVEN script installs to `~/.local/bin`
- AND `~/.local/bin` is not in PATH
- WHEN installation completes successfully
- THEN warning message is displayed
- AND message includes instructions to add directory to PATH

#### Scenario: Directory already in PATH
- GIVEN script installs to `~/.local/bin`
- AND `~/.local/bin` is in PATH
- WHEN installation completes successfully
- THEN no PATH warning is displayed

### REQ-14: System-wide installation flag
The install script MUST accept a `--system` flag to install to `/usr/local/bin` with sudo.

#### Scenario: System-wide installation
- GIVEN user runs `./install.sh --system`
- WHEN script runs
- THEN binary is installed to `/usr/local/bin/glpictl-ai`
- AND sudo is required if user lacks permissions
- AND no PATH warning is shown

## MODIFIED Requirements

### REQ-8: Install Script (Linux/macOS)
`install.sh` MUST:
- Detect OS and architecture
- Download the appropriate binary from GitHub releases
- Place it in `~/.local/bin/glpictl-ai` by default
- Make it executable
- Run `glpictl-ai configure`
- Accept `--dir <path>` flag for custom installation directory
- Accept `--system` flag for `/usr/local/bin` installation
- Check if installation directory exists, create if needed
- Display PATH warning if installation directory not in PATH
(Previously: Always installed to `/usr/local/bin/glpictl-ai` and required sudo)

#### Scenario: Default user-local install
- GIVEN user runs `./install.sh` with no flags
- WHEN script completes
- THEN binary is installed to `~/.local/bin/glpictl-ai`
- AND directory was created if needed
- AND PATH warning shown if not in PATH
- AND configure command runs

#### Scenario: System-wide install with flag
- GIVEN user runs `sudo ./install.sh --system`
- WHEN script completes
- THEN binary is installed to `/usr/local/bin/glpictl-ai`
- AND no PATH warning is shown
- AND configure command runs

#### Scenario: Custom directory install
- GIVEN user runs `./install.sh --dir ~/bin`
- WHEN script completes
- THEN binary is installed to `~/bin/glpictl-ai`
- AND PATH warning shown if `~/bin` not in PATH
- AND configure command runs

#### Scenario: Binary download and install
- GIVEN user runs `./install.sh`
- WHEN script runs
- THEN OS and architecture are detected
- AND appropriate binary is downloaded from GitHub releases
- AND binary is installed to specified directory
- AND binary is made executable
- AND configure command runs

## REMOVED Requirements

None. All existing requirements are preserved or modified to support new behavior.
