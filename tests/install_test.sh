#!/bin/sh
# Offline integration checks: real archives, mocked GitHub downloads.
set -eu
repo_dir=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
fixture=$(mktemp -d)
trap 'rm -rf "$fixture"' EXIT
export FIXTURE="$fixture"
mkdir -p "$fixture/mock" "$fixture/source" "$fixture/bin with spaces"
cat > "$fixture/mock/curl" <<'MOCK'
#!/bin/sh
set -eu
out=
url=
while [ "$#" -gt 0 ]; do
    case "$1" in
        -o) out=$2; shift 2 ;;
        --proto|--proto-redir|--retry|-w) shift 2 ;;
        -*) shift ;;
        *) url=$1; shift ;;
    esac
done
if [ "${FAIL_DOWNLOAD:-}" = 1 ]; then exit 22; fi
case "$url" in
    */releases/latest) printf 'https://github.com/OliverChoi-iOS/redmine-cli/releases/tag/v0.1.0' ;;
    */download/v0.1.0/*) cp "$FIXTURE/${url##*/}" "$out" ;;
    *) echo "Unexpected URL: $url" >&2; exit 1 ;;
esac
MOCK
cat > "$fixture/mock/uname" <<'MOCK'
#!/bin/sh
case "$1" in
    -s) printf '%s\n' "${TEST_OS:-Linux}" ;;
    -m) printf '%s\n' "${TEST_ARCH:-x86_64}" ;;
esac
MOCK
chmod +x "$fixture/mock/curl" "$fixture/mock/uname"
printf '#!/bin/sh\nprintf "0.1.0\\n"\n' > "$fixture/source/redmine"
chmod +x "$fixture/source/redmine"
for os in linux darwin; do
    for arch in amd64 arm64; do
        archive="redmine-cli_0.1.0_${os}_${arch}.tar.gz"
        tar -czf "$fixture/$archive" -C "$fixture/source" redmine
        (cd "$fixture" && shasum -a 256 "$archive") >> "$fixture/checksums.txt"
    done
done
export PATH="$fixture/mock:$PATH"
export INSTALL_DIR="$fixture/bin with spaces"
VERSION=0.1.0 sh "$repo_dir/install.sh"
[ "$("$INSTALL_DIR/redmine" --version)" = 0.1.0 ]
TEST_OS=Darwin TEST_ARCH=arm64 VERSION='' sh "$repo_dir/install.sh"
TEST_ARCH=aarch64 VERSION=v0.1.0 sh "$repo_dir/install.sh"
original=$(shasum -a 256 "$INSTALL_DIR/redmine")
expect_failure() {
    if "$@" > "$fixture/error.log" 2>&1; then echo 'Expected installer failure' >&2; exit 1; fi
    [ "$(shasum -a 256 "$INSTALL_DIR/redmine")" = "$original" ]
}
expect_failure env VERSION=bad sh "$repo_dir/install.sh"
expect_failure env TEST_ARCH=i686 sh "$repo_dir/install.sh"
expect_failure env TEST_OS=FreeBSD sh "$repo_dir/install.sh"
expect_failure env VERSION=0.1.0 FAIL_DOWNLOAD=1 sh "$repo_dir/install.sh"
printf 'corrupt' >> "$fixture/redmine-cli_0.1.0_linux_amd64.tar.gz"
expect_failure env VERSION=0.1.0 sh "$repo_dir/install.sh"
grep -q 'Checksum verification failed' "$fixture/error.log"
printf '' > "$fixture/checksums.txt"
expect_failure env VERSION=0.1.0 sh "$repo_dir/install.sh"
grep -q 'Missing or invalid checksum' "$fixture/error.log"
printf '#!/bin/sh\nprintf "wrong-version\\n"\n' > "$fixture/source/redmine"
tar -czf "$fixture/redmine-cli_0.1.0_linux_amd64.tar.gz" -C "$fixture/source" redmine
(cd "$fixture" && shasum -a 256 redmine-cli_0.1.0_linux_amd64.tar.gz) > "$fixture/checksums.txt"
expect_failure env VERSION=0.1.0 sh "$repo_dir/install.sh"
grep -q 'Binary version check failed' "$fixture/error.log"
echo 'Unix installer checks passed.'
