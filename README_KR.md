# redmine-cli

[English](README.md) | [Korean](README_KR.md)

AI 에이전트가 Redmine 이슈 작업을 안전하고 일관되게 실행할 수 있도록 만든 CLI입니다.

이 프로젝트는 Codex 같은 AI 에이전트를 위한 이슈 중심 CLI입니다. 기본 인터페이스는 비대화형, JSON 우선이며, 안정적인 종료 코드, 안정적인 오류 코드, 명시적인 쓰기 위험도, Redmine 문법 댓글을 중심으로 설계되어 있습니다.

## 사전 요구사항

- Go 1.24. `mise x go@1.24 -- ...`로 실행하거나, `PATH`에 직접 설치된 `go`를 사용할 수 있습니다. 현재 macOS 기준 검증된 개발 도구체인은 Go 1.24이며, `go.mod`의 언어 호환성 선언은 Go 1.22를 유지합니다.
- 이 README의 명령을 그대로 실행하려면 `mise`가 `PATH`에 있어야 합니다.
- `redmine` 바이너리를 둘 디렉터리가 `PATH`에 있어야 합니다. 빠른 시작에서는 `~/.local/bin`과 `export PATH="$HOME/.local/bin:$PATH"`를 사용합니다.
- 쓰기 가능한 Go 빌드 캐시가 필요합니다. 제한된 셸에서는 Go 명령 앞에 `GOCACHE=/tmp/redmine-cli-go-cache`를 붙이세요.
- Redmine 기본 URL, API 키, Redmine 서버에 접근할 수 있는 네트워크가 필요합니다.
- 실행하려는 Redmine 작업에 필요한 권한을 가진 API 키가 필요합니다.
- 일반 사용에서는 `https://` Redmine URL을 사용해야 합니다. `http://`는 기본 차단되며, 로컬 fake/test 서버에서만 `--allow-insecure-http`로 1회성 허용할 수 있습니다.

### mise와 Go 설치

macOS 권장 설치:

```bash
brew install mise
mise install go@1.24
mise x go@1.24 -- go version
```

Homebrew를 사용할 수 없다면 사용하는 셸에 맞는 공식 mise installer를 사용하세요:

```bash
curl https://mise.run/zsh | sh
exec zsh
mise install go@1.24
mise x go@1.24 -- go version
```

mise 없이 Go를 직접 설치하려면:

```bash
brew install go
go version
```

Homebrew의 Go 패키지는 검증된 Go 1.24보다 새로운 버전을 설치할 수 있습니다. README 명령을 정확히 재현해야 한다면 `mise x go@1.24 -- ...` 명령을 사용하세요.

공식 참고 문서:

- [mise installation methods](https://mise.jdx.dev/installing-mise.html)
- [Go download and install guide](https://go.dev/doc/install)

## 빠른 시작

CLI를 빌드하고 한 번 로그인한 뒤, 매번 인증 환경변수를 설정하지 않고 Redmine 명령을 실행합니다.

### 1. CLI 빌드

저장소 루트에서 실행합니다:

```bash
GOCACHE=/tmp/redmine-cli-go-cache mise x go@1.24 -- go build -o redmine ./cmd/redmine
mkdir -p "$HOME/.local/bin"
install -m 755 redmine "$HOME/.local/bin/redmine"
export PATH="$HOME/.local/bin:$PATH"
redmine --help
redmine --version
```

명시적인 `GOCACHE`는 기본 Go 캐시에 쓸 수 없는 제한된 셸에서도 빌드가 되도록 합니다. 일반 Go 캐시에 쓸 수 있는 환경이라면 생략해도 됩니다.

`export PATH=...` 줄은 현재 셸에서 `redmine` 명령을 찾을 수 있게 합니다. 새 터미널에서도 계속 사용하려면 `~/.local/bin`이 아직 `PATH`에 없을 때 같은 줄을 `~/.zshrc` 같은 shell profile에 추가하세요.

### 2. 한 번 로그인

예시 URL을 실제 Redmine 설치의 기본 URL로 바꾸세요:

```bash
redmine auth login --url https://redmine.example.com
redmine auth status --format pretty-json
```

`auth login`은 API 키를 검증한 뒤 URL과 키를 `0600` 권한의 CLI 설정 파일에 저장합니다. 성공한 status 응답에는 `"ok": true`와 `"authenticated": true`가 포함됩니다.

Redmine URL은 기본적으로 `https://`를 사용해야 합니다. 로컬 테스트 서버에서만 해당 명령 1회에 `--allow-insecure-http`를 전달하세요.

### 3. 첫 조회 실행

```bash
redmine issue list --limit 5 --format pretty-json
```

로그인 후에는 `REDMINE_URL`과 `REDMINE_API_KEY`가 필요하지 않습니다. 목록이 완전하다고 판단하기 전에 `pagination.has_more`를 확인하세요.

## 지원 명령

포함된 명령:

- `redmine issue list`
- `redmine issue get`
- `redmine issue create`
- `redmine issue update`
- `redmine issue comment`
- `redmine issue assign`
- `redmine issue transition`
- `redmine issue close`
- `redmine issue reopen`
- `redmine issue link`
- `redmine project list`
- `redmine project get`
- `redmine user me`
- `redmine user search`
- `redmine meta sync`
- `redmine meta statuses`
- `redmine meta trackers`
- `redmine meta priorities`
- `redmine meta custom-fields`
- `redmine config init`
- `redmine config show`
- `redmine auth login`
- `redmine auth logout`
- `redmine auth status`

## 제약사항

- HTTPS가 기본 요구사항입니다. `--allow-insecure-http`는 로컬/테스트용 1회성 예외이며, API 키가 평문 HTTP로 전송될 수 있다는 경고를 출력합니다.
- 인증 정보는 `0600` 권한의 CLI 설정 파일에 저장됩니다. OS keychain 연동은 아직 지원하지 않습니다.
- Redmine 서버/프로필은 하나만 지원합니다. 여러 Redmine 프로필은 아직 지원하지 않습니다.
- 이슈 첨부파일 업로드/다운로드를 포함한 파일 업로드는 아직 지원하지 않습니다.
- Admin API, wiki 편집, time tracking, repository API, bulk/destructive operation은 아직 지원하지 않습니다.
- 댓글은 Redmine 문법으로 전송됩니다. Markdown에서 Redmine 문법으로 자동 변환하지 않습니다.
- `.env` 파일은 자동으로 로드하지 않습니다. 로컬에서는 `redmine auth login`을 사용하고, CI 또는 1회성 override에서는 `REDMINE_URL`과 `REDMINE_API_KEY`를 사용하세요.

## 설계 기본값

- 기본 출력은 JSON입니다.
- `--format table`은 사람이 보기 위한 출력이며 안정적인 API가 아닙니다.
- `stdout`은 기계가 읽을 수 있는 명령 결과 전용입니다.
- `stderr`는 진단 메시지, 경고, 진행 상황에 사용합니다.
- 댓글은 Redmine 문법을 사용합니다. Markdown 변환은 의도적으로 지원하지 않습니다.
- 쓰기 명령은 위험도에 따라 구분됩니다.
  - 낮은 위험도의 쓰기는 즉시 실행됩니다.
  - 중간 위험도의 쓰기는 즉시 실행되며 가능한 경우 변경 전/후를 반환합니다.
  - 높은 위험도의 쓰기는 보류되거나 향후 릴리스에서 명시적인 확인을 요구합니다.
- metadata cache가 포함되어 있습니다.
  - TTL 기본값은 24시간입니다.
  - 이름 해석에 실패하면 한 번 refresh를 시도합니다.
  - JSON metadata에 cache 상태가 포함됩니다.

## 설정

이 CLI는 현재 하나의 Redmine 서버를 대상으로 합니다.

로컬 사용에서는 저장형 로그인 흐름을 권장합니다:

```bash
redmine auth login --url https://redmine.example.com
redmine config show --format pretty-json
redmine auth status --format pretty-json
```

`config show`는 API 키를 출력하지 않고 설정 파일 경로와 인증 정보 출처를 보고합니다.

인증 정보 해석 순서:

1. CLI 플래그: `--url`, `--api-key`
2. 환경변수: `REDMINE_URL`, `REDMINE_API_KEY`
3. `redmine auth login`이 작성한 설정 파일
4. Keychain 지원: 아직 지원하지 않음

환경변수와 플래그는 CI 또는 1회성 override 용도입니다. 해당 실행에서 저장된 URL 또는 API 키만 대체하며, 설정 파일의 다른 값은 유지됩니다. `http://` URL은 해당 실행에 `--allow-insecure-http`를 전달하지 않으면 거부됩니다.

CI에서는 명령이나 파일에 실제 키를 쓰지 말고 CI secret store를 통해 `REDMINE_API_KEY`를 주입하세요:

```bash
export REDMINE_URL="https://redmine.example.com"
# REDMINE_API_KEY는 CI secret store가 주입합니다.
redmine issue list --limit 5 --format pretty-json
```

API 키를 `.env` 파일, 스크립트, 예시, 명령 기록, 프로세스 인자에 남기지 마세요. 실제 값을 `--api-key`에 직접 전달하는 것보다 `redmine auth login`, `REDMINE_API_KEY`, 또는 `auth login --stdin`을 선호하세요. 이 CLI는 `.env` 파일을 자동으로 로드하지 않습니다.

`redmine auth login`은 GitLab CLI의 파일 기반 인증 정보 저장 방식과 비슷하게 URL과 API 키를 `0600` 권한의 설정 파일에 저장합니다. `REDMINE_URL`, `REDMINE_API_KEY`, `--url`, `--api-key`는 저장된 인증 정보를 해당 실행 1회에만 override합니다. `--api-key`는 명령행 인자가 shell history와 process listing에 노출될 수 있으므로 안전하지 않은 escape hatch입니다. `redmine auth logout`은 저장된 API 키를 제거합니다.

다음 예시는 secret이 아닌 설정 값만 보여줍니다. API 키는 `auth login`이 추가하므로 직접 literal 값을 넣지 마세요.

```toml
url = "https://redmine.example.com"
default_project = "mobile"
metadata_ttl = "24h"
default_format = "json"
append_footer = false
close_status = "Closed"
reopen_status = "New"
```

## 개발

CLI는 Go 표준 라이브러리로 구현되어 있으며, 주요 명령 계약 테스트에는 fake Redmine 서버를 사용합니다.

Go를 선택한 이유는 이 CLI가 여러 환경에서 에이전트에 의해 빠르게 설치되고 안정적으로 호출되어야 하기 때문입니다. 단일 바이너리 배포는 런타임 설정 부담을 줄이고, Go 표준 라이브러리의 `net/http`, `encoding/json`, cross-compilation 지원, `httptest`만으로도 typed Redmine client, 안정적인 JSON 계약, fake Redmine server 테스트를 충분히 구현할 수 있습니다.

```bash
GOCACHE=/tmp/redmine-cli-go-cache mise x go@1.24 -- go test ./...
GOCACHE=/tmp/redmine-cli-go-cache mise x go@1.24 -- go run ./cmd/redmine --help
GOCACHE=/tmp/redmine-cli-go-cache mise x go@1.24 -- go run ./cmd/redmine --version
```

구현은 `schema/output.schema.json`의 JSON 출력 계약을 안정적으로 유지해야 합니다.

## 문제 해결

### `go: command not found`

검증된 mise toolchain으로 Go를 실행하세요:

```bash
mise x go@1.24 -- go version
```

### `redmine: command not found`

빌드한 바이너리를 `PATH`에 있는 디렉터리에 설치한 뒤, 현재 셸의 `PATH`를 갱신하세요:

```bash
mkdir -p "$HOME/.local/bin"
install -m 755 redmine "$HOME/.local/bin/redmine"
export PATH="$HOME/.local/bin:$PATH"
redmine --version
```

새 터미널에서도 `redmine`을 자동으로 찾게 하려면 `export PATH="$HOME/.local/bin:$PATH"`를 `~/.zshrc` 같은 shell profile에 추가하세요.

### Go cache `operation not permitted`

`GOCACHE`를 쓰기 가능한 임시 디렉터리로 지정하세요:

```bash
GOCACHE=/tmp/redmine-cli-go-cache mise x go@1.24 -- go test ./...
```

### macOS `missing LC_UUID load command`

현재 macOS에서 빌드와 테스트에는 Go 1.24를 사용하세요:

```bash
GOCACHE=/tmp/redmine-cli-go-cache mise x go@1.24 -- go test ./...
```

### `http://` Redmine URL에서 `validation_failed`

인증 명령에는 `https://` Redmine URL을 사용하세요. 테스트의 로컬 fake Redmine 서버에서만 해당 실행에 `--allow-insecure-http`를 전달하세요.

### `config_missing`, `auth_missing`, 또는 `auth_failed`

저장된 인증 정보를 갱신하고 검증한 뒤 명령을 다시 실행하세요:

```bash
redmine auth login --url https://redmine.example.com
redmine auth status --format pretty-json
```

1회성 명령에서는 `REDMINE_URL`과 `REDMINE_API_KEY`, 또는 대응하는 `--url`과 `--api-key` 플래그를 사용하세요.
