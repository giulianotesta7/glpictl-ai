# Tasks: installer-no-sudo - Remove sudo requirement from installer

## Phase 1: Infrastructure & Argument Parsing

- [x] 1.1 Add argument parsing for --dir and --system flags in install.sh
- [x] 1.2 Add INSTALL_DIR variable initialization with ~/.local/bin default
- [x] 1.3 Update INSTALL_DIR logic: --system flag overrides default to /usr/local/bin

## Phase 2: Core Implementation

- [x] 2.1 Create ensure_install_dir() function: mkdir -p + writability check
- [x] 2.2 Create check_path() function: check if INSTALL_DIR in PATH, warn if not
- [x] 2.3 Remove check_root() function call (no longer required by default)
- [x] 2.4 Update install_binary() to call ensure_install_dir() before install
- [x] 2.5 Update install_binary() to check writability, fail with clear error
- [x] 2.6 Update run_configure() to handle user-local installs correctly

## Phase 3: Testing

- [ ] 3.1 Test default install to ~/.local/bin (no sudo required)
- [ ] 3.2 Test --dir flag with custom path creation
- [ ] 3.3 Test --system flag with /usr/local/bin (requires sudo)
- [ ] 3.4 Test PATH warning when ~/.local/bin not in PATH
- [ ] 3.5 Test error message when directory not writable
- [ ] 3.6 Verify configure command runs as correct user

## Phase 4: Documentation

- [ ] 4.1 Update install.sh usage comments to reflect new flags
- [ ] 4.2 Update README with installation examples for all modes
