#!/bin/sh
# Usage: VERSION=v0.1.0 INSTALL_DIR="$HOME/.local/bin" sh install.sh
main() {
    set -eu
    [ "$#" -eq 0 ] || { echo 'Set VERSION and INSTALL_DIR as environment variables.' >&2; exit 1; }
    repo=https://github.com/OliverChoi-iOS/redmine-cli
    install_dir=${INSTALL_DIR:-"$HOME/.local/bin"}
    for tool in curl tar uname mktemp install; do
        command -v "$tool" >/dev/null 2>&1 || { echo "Required tool not found: $tool" >&2; exit 1; }
    done
    if command -v sha256sum >/dev/null 2>&1; then
        hash_tool=sha256sum
    elif command -v shasum >/dev/null 2>&1; then
        hash_tool=shasum
    else
        echo 'Install sha256sum or shasum to verify downloads.' >&2; exit 1
    fi
    case "$(uname -s)" in
        Darwin) os=darwin ;;
        Linux) os=linux ;;
        *) echo 'Unsupported OS. On Windows, use install.ps1.' >&2; exit 1 ;;
    esac
    case "$(uname -m)" in
        x86_64|amd64) arch=amd64 ;;
        arm64|aarch64) arch=arm64 ;;
        *) echo 'Unsupported CPU. Supported: amd64, arm64.' >&2; exit 1 ;;
    esac
    tag=${VERSION:-}
    if [ -z "$tag" ]; then
        latest=$(curl --proto '=https' --proto-redir '=https' -fsSL --retry 3 -o /dev/null -w '%{url_effective}' "$repo/releases/latest")
        tag=${latest##*/}
    fi
    case "$tag" in v*) ;; *) tag="v$tag" ;; esac
    printf '%s\n' "$tag" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$' || {
        echo 'Expected a stable version such as v0.1.0. Check that a release exists.' >&2; exit 1;
    }
    version=${tag#v}
    archive="redmine-cli_${version}_${os}_${arch}.tar.gz"
    download="$repo/releases/download/$tag"
    tmp_dir=$(mktemp -d)
    staged_binary=
    trap 'rm -rf "$tmp_dir"; if [ -n "$staged_binary" ]; then rm -f "$staged_binary"; fi' EXIT
    trap 'exit 1' HUP INT TERM
    curl --proto '=https' --proto-redir '=https' -fsSL --retry 3 "$download/$archive" -o "$tmp_dir/$archive"
    curl --proto '=https' --proto-redir '=https' -fsSL --retry 3 "$download/checksums.txt" -o "$tmp_dir/checksums.txt"
    expected=$(awk -v name="$archive" '$2 == name { print $1 }' "$tmp_dir/checksums.txt")
    [ "${#expected}" -eq 64 ] || { echo 'Missing or invalid checksum.' >&2; exit 1; }
    if [ "$hash_tool" = sha256sum ]; then
        actual=$(sha256sum "$tmp_dir/$archive")
    else
        actual=$(shasum -a 256 "$tmp_dir/$archive")
    fi
    actual=${actual%% *}
    [ "$actual" = "$expected" ] || { echo 'Checksum verification failed.' >&2; exit 1; }
    tar -xzf "$tmp_dir/$archive" -C "$tmp_dir" redmine
    [ "$("$tmp_dir/redmine" --version)" = "$version" ] || { echo 'Binary version check failed.' >&2; exit 1; }
    mkdir -p "$install_dir"
    [ ! -d "$install_dir/redmine" ] || { echo 'Install target is a directory.' >&2; exit 1; }
    staged_binary=$(mktemp "$install_dir/.redmine.XXXXXX")
    install -m 755 "$tmp_dir/redmine" "$staged_binary"
    mv -f "$staged_binary" "$install_dir/redmine"
    staged_binary=
    printf 'Installed redmine %s to %s/redmine\n' "$version" "$install_dir"
    case ":$PATH:" in
        *":$install_dir:"*) ;;
        *) printf 'Add this directory to your shell profile: export PATH="%s:$PATH"\n' "$install_dir" ;;
    esac
}

main "$@"
