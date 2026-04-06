# Proposal: Remove sudo requirement from installer

## Intent

Remove the `sudo` requirement from `install.sh` by defaulting to `~/.local/bin` instead of `/usr/local/bin`.

## Scope

- Change default installation directory from `/usr/local/bin` to `~/.local/bin`
- Add check to create `~/.local/bin` if it doesn't exist
- Add `--dir` flag to allow custom installation directory
- Display PATH warning if `~/.local/bin` is not in PATH
- Maintain backward compatibility for users who want system-wide installation

## Approach

1. Modify `install.sh` to:
   - Default to `~/.local/bin` as installation directory
   - Check if directory exists, create it if needed (without sudo)
   - Add `--dir <path>` flag for custom directory
   - Detect if `~/.local/bin` is in PATH and warn if missing
   - Keep `--system` or `--sudo` flag for users who want `/usr/local/bin`

2. User experience:
   - Default: `./install.sh` → installs to `~/.local/bin`
   - Custom dir: `./install.sh --dir /custom/path`
   - System-wide: `./install.sh --system` → requires sudo

3. Error handling:
   - Fail gracefully if directory cannot be created
   - Show clear instructions for PATH setup

## Impact

- No breaking changes for existing users (can still use system-wide install)
- Removes friction for new users without sudo access
- Aligns with XDG Base Directory specification for user-local binaries
