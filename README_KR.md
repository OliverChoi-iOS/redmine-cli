# redmine-cli

[English](README.md) | [Korean](README_KR.md)

AI 에이전트가 Redmine 이슈 작업을 안전하고 일관되게 실행할 수 있도록 만든 CLI입니다.

이 프로젝트는 Codex 같은 AI 에이전트를 위한 이슈 중심 CLI입니다. 기본 인터페이스는 비대화형, JSON 우선이며, 안정적인 종료 코드, 안정적인 오류 코드, 명시적인 쓰기 위험도, Redmine 문법 댓글을 중심으로 설계되어 있습니다.

## 사전 요구사항

- 릴리스 바이너리 사용에는 Go나 mise가 필요하지 않습니다. Go는 [개발](#개발)할 때만 필요합니다.
- `redmine` 바이너리를 둘 디렉터리가 `PATH`에 있어야 합니다. 빠른 시작에서는 `~/.local/bin`과 `export PATH="$HOME/.local/bin:$PATH"`를 사용합니다.
- macOS/Linux: `curl`, `tar`, `sha256sum` 또는 `shasum`이 필요합니다. Windows: PowerShell 5.1 이상이 필요합니다.
- Redmine 기본 URL, API 키, Redmine 서버에 접근할 수 있는 네트워크가 필요합니다.
- 실행하려는 Redmine 작업에 필요한 권한을 가진 API 키가 필요합니다.
- 일반 사용에서는 `https://` Redmine URL을 사용해야 합니다. `http://`는 기본 차단되며, 로컬 fake/test 서버에서만 `--allow-insecure-http`로 1회성 허용할 수 있습니다.

## 빠른 시작

CLI를 설치하고 한 번 로그인한 뒤 Redmine 명령을 실행합니다.

### 1. CLI 설치

macOS와 Linux (Intel/AMD64 또는 ARM64):

```bash
curl -fsSL https://github.com/OliverChoi-iOS/redmine-cli/releases/latest/download/install.sh -o install.sh
sh install.sh
export PATH="$HOME/.local/bin:$PATH"
redmine --version
```

설치 스크립트는 압축 파일의 SHA-256 체크섬과 바이너리 버전을 확인한 뒤 `~/.local/bin`에 설치합니다. 다운로드한 스크립트는 실행 전에 확인하세요. 다시 실행하면 업데이트되며 인증 설정은 유지됩니다. 새 터미널에서도 사용하려면 `export PATH=...` 줄을 `~/.zshrc` 같은 셸 설정 파일에 추가하세요.

버전이나 설치 경로를 지정할 수도 있습니다:

```bash
VERSION=v0.1.0 INSTALL_DIR="$HOME/.local/bin" sh install.sh
```

Windows PowerShell (AMD64 또는 ARM64):

```powershell
Invoke-WebRequest https://github.com/OliverChoi-iOS/redmine-cli/releases/latest/download/install.ps1 -OutFile install.ps1
& ./install.ps1
$env:Path = "$env:LOCALAPPDATA\redmine-cli\bin;$env:Path"
redmine --version
```

기본 설치 위치는 `%LOCALAPPDATA%\redmine-cli\bin`입니다. 새 터미널에서도 사용하려면 사용자 PATH에 이 경로를 추가하세요. `./install.ps1 -Version v0.1.0 -InstallDir C:\Tools\redmine`으로 버전과 경로를 지정할 수 있습니다. 스크립트 실행이 제한된 환경에서는 조직의 PowerShell 실행 정책을 따르세요.

Windows 지원은 실험적입니다. 현재 인증 저장 코드는 Unix 파일 권한을 전제로 하므로, Windows에서는 `auth login` 대신 `REDMINE_URL`과 안전하게 주입한 `REDMINE_API_KEY`를 사용하세요.

[Releases](https://github.com/OliverChoi-iOS/redmine-cli/releases)에서 압축 파일을 직접 다운로드하고 `checksums.txt`와 대조한 뒤 `redmine` 또는 `redmine.exe`를 PATH에 설치할 수도 있습니다. 다운로드는 첫 정식 릴리스를 게시한 후 사용할 수 있습니다. 체크섬은 다운로드 손상을 검출하며 릴리스 서명은 아직 구성하지 않았습니다.

삭제하려면 설치된 바이너리를 지우세요. 설정과 캐시는 남습니다.

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
- `redmine issue attach`
- `redmine wiki list`
- `redmine wiki get`
- `redmine wiki put`
- `redmine time create`
- `redmine time activities`
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

## 위키 작성과 이슈 첨부파일 업로드

`--project`에는 프로젝트 식별자 또는 숫자 ID를 사용합니다. 생략하면 `default_project`를 사용합니다.

```bash
redmine wiki list --project mobile
redmine wiki get "설치_가이드" --project mobile --include attachments
redmine wiki put "설치_가이드" --project mobile --text-file guide.redmine --comments "설치 절차 갱신" --dry-run
# --dry-run을 빼면 페이지를 생성하거나 수정합니다.
redmine wiki put "설치_가이드" --project mobile --text-file guide.redmine --page-version 4
redmine issue attach 123 --file evidence.png --description "재현 화면" --dry-run
# --dry-run을 빼면 파일을 업로드하고 이슈에 첨부합니다.
```

`wiki put`은 Redmine 위키 문법의 본문을 변환 없이 전송합니다. 페이지가 없으면 생성하고, 있으면 본문을 교체합니다. 수정 시 현재 버전을 조회해 함께 전송하므로 명령 실행 중 발생한 동시 수정을 감지합니다. 이전에 읽은 뒤 발생한 변경까지 보호하려면 읽었던 버전을 `--page-version N`으로 지정하세요. 버전이 다르면 `conflict` 오류와 종료 코드 7을 반환합니다. `wiki get --page-version N`은 과거 버전을 조회합니다. 기존 상위 페이지는 유지되며, `--parent-title`로 바꾸거나 `--parent-title ""`로 루트 페이지로 만들 수 있습니다. 생성과 수정이 같은 PUT API를 사용하므로 새 페이지 생성 시 다른 작성자와의 경합 가능성은 남아 있습니다.

`issue attach`는 비어 있지 않은 일반 파일 하나를 업로드합니다. `--filename`으로 서버에 저장할 이름을 지정하고, `--content-type`으로 확장자에서 추론한 MIME 형식을 덮어쓸 수 있습니다. 추론할 수 없으면 `application/octet-stream`을 사용합니다. 서버의 파일 크기 제한과 권한이 적용됩니다. Redmine 7.1 이상은 서버에서 파일 형식을 감지합니다. 업로드 후 이슈 연결이 실패하면 `error.details.upload_completed`와 `attachment_link_confirmed: false`를 반환합니다. 응답 유실로 이미 첨부됐을 수도 있으므로 재시도 전에 이슈를 확인하세요. 재실행은 파일을 다시 업로드하며, 연결 실패 시 서버에 미사용 업로드가 남을 수 있습니다.

두 쓰기 명령의 `--dry-run`은 서버에 접속하지 않고 로컬 입력만 검증합니다. 서버 권한, 자동 버전 조회, 기존 상위 페이지 확인은 실제 실행 시 수행합니다. 쓰기 성공 후에는 가능한 경우 `before`와 `after`를 반환합니다. 마지막 재조회만 실패했다면 `ok: true`와 경고를 반환하므로 결과 확인을 위해 쓰기를 반복하지 마세요.

공식 API 문서: [위키 페이지](https://www.redmine.org/projects/redmine/wiki/Rest_WikiPages), [파일 첨부](https://www.redmine.org/projects/redmine/wiki/Rest_api#Attaching-files).

## 소요시간 등록

작업유형 ID를 확인한 뒤 이슈에 실제 작업시간을 등록합니다. 로그인한 사용자 명의로 기록됩니다.

```bash
redmine time activities --format pretty-json
redmine time create --issue 123 --hours 1.5 --activity-id 9 --spent-on 2026-11-05 --comments "로그인 오류 수정" --dry-run
# 9를 실제 서버의 작업유형 ID로 바꾸세요. --dry-run을 빼면 등록합니다.
```

`--issue`, `--hours`, `--activity-id`, `--spent-on`은 필수입니다. 시간은 0보다 큰 유한한 숫자이며 단위는 시간입니다. `1.5`는 1시간 30분이고, 날짜는 `YYYY-MM-DD` 형식입니다. 서버의 기본 날짜나 작업유형이 잘못 적용되지 않도록 두 값을 명시합니다. 설명은 선택이며 `--comments` 또는 `--comments-file PATH`로 최대 255자까지 입력합니다. 파일의 줄바꿈도 글자 수에 포함됩니다. `--dry-run`은 서버에 접속하지 않고 입력을 검증해 `data.request`로 보여줍니다.

등록 성공 시 생성된 기록의 ID, 시간, 작업일, 사용자, 작업유형을 반환합니다. 실제 소요시간을 기록하는 기능이며 예상시간인 `estimated_hours`는 변경하지 않습니다. 시간 등록 권한, 프로젝트별 작업유형, 필수 사용자 정의 필드, 시간·날짜 제한은 Redmine 서버가 검증합니다. 작업유형은 메타데이터 캐시 대신 `/enumerations/time_entry_activities.json`에서 직접 조회합니다.

실행할 때마다 새 기록을 생성하며 자동 재시도하지 않습니다. 네트워크 오류나 응답 유실이 발생하면 중복 등록을 피하도록 Redmine의 해당 이슈에서 소요시간 내역을 확인한 뒤 재시도하세요. 소요시간 내역 조회·수정·삭제, 이슈 없는 프로젝트 단위 등록, 다른 사용자 명의 등록은 아직 지원하지 않습니다.

공식 API 문서: [소요시간 기록](https://www.redmine.org/projects/redmine/wiki/Rest_TimeEntries), [작업유형 조회](https://www.redmine.org/projects/redmine/wiki/Rest_Enumerations#enumerationstime_entry_activitiesformat).

## 제약사항

- HTTPS가 기본 요구사항입니다. `--allow-insecure-http`는 로컬/테스트용 1회성 예외이며, API 키가 평문 HTTP로 전송될 수 있다는 경고를 출력합니다.
- 인증 정보는 `0600` 권한의 CLI 설정 파일에 저장됩니다. OS keychain 연동은 아직 지원하지 않습니다.
- Redmine 서버/프로필은 하나만 지원합니다. 여러 Redmine 프로필은 아직 지원하지 않습니다.
- 파일 다운로드, 위키 첨부, 프로젝트의 ‘파일’ 등록은 아직 지원하지 않습니다. `issue attach`는 한 번에 파일 하나를 업로드합니다.
- Admin API, repository API, bulk/destructive operation은 아직 지원하지 않습니다. 소요시간 등록은 지원하며 내역 조회·수정·삭제는 지원하지 않습니다.
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

검증된 개발 도구체인은 Go 1.24이며 `go.mod`는 Go 1.22 언어 호환성을 유지합니다. [Go 설치 가이드](https://go.dev/doc/install)를 따라 직접 설치하거나 [mise](https://mise.jdx.dev/installing-mise.html)를 사용하세요:

```bash
brew install mise
mise install go@1.24
GOCACHE=/tmp/redmine-cli-go-cache mise x go@1.24 -- go build -o redmine ./cmd/redmine
./redmine --version
```

CLI는 Go 표준 라이브러리로 구현되어 있으며, 주요 명령 계약 테스트에는 fake Redmine 서버를 사용합니다.

Go를 선택한 이유는 이 CLI가 여러 환경에서 에이전트에 의해 빠르게 설치되고 안정적으로 호출되어야 하기 때문입니다. 단일 바이너리 배포는 런타임 설정 부담을 줄이고, Go 표준 라이브러리의 `net/http`, `encoding/json`, cross-compilation 지원, `httptest`만으로도 typed Redmine client, 안정적인 JSON 계약, fake Redmine server 테스트를 충분히 구현할 수 있습니다.

```bash
GOCACHE=/tmp/redmine-cli-go-cache mise x go@1.24 -- go test ./...
GOCACHE=/tmp/redmine-cli-go-cache mise x go@1.24 -- go run ./cmd/redmine --help
GOCACHE=/tmp/redmine-cli-go-cache mise x go@1.24 -- go run ./cmd/redmine --version
```

구현은 `schema/output.schema.json`의 JSON 출력 계약을 안정적으로 유지해야 합니다.

### 릴리스 게시

릴리스 워크플로와 설치 스크립트를 `main`에 커밋한 뒤 새 정식 버전 태그를 푸시하세요. `v0.1.0`은 아직 사용하지 않은 다음 버전으로 바꾸세요:

```bash
git push origin main
git tag v0.1.0
git push origin v0.1.0
```

[Release Actions](https://github.com/OliverChoi-iOS/redmine-cli/actions/workflows/release.yml)가 macOS·Linux·Windows에서 테스트하고, CGO 없이 6종 바이너리(darwin/linux/windows × amd64/arm64)를 빌드합니다. 압축 파일, 두 설치 스크립트, `checksums.txt`, 자동 생성 릴리스 노트를 게시하며 `redmine --version`은 태그에서 `v`를 뺀 값을 출력합니다. Unix에서는 전체 Go 테스트를 실행하고, Windows에서는 Unix 권한을 전제로 하는 인증 저장 테스트 대신 이식 가능한 패키지·버전 명령·설치 스크립트를 검증합니다. `vMAJOR.MINOR.PATCH` 형식의 정식 버전만 지원합니다.

워크플로는 `contents: write` 권한의 기본 `GITHUB_TOKEN`을 사용하므로 별도 PAT가 필요하지 않습니다. 게시 도중 실패하면 초안 릴리스가 남을 수 있으니 GitHub Releases에서 확인한 뒤 재시도하세요. 게시된 릴리스를 덮어쓰지는 않습니다.

설치 스크립트의 오프라인 테스트는 `sh tests/install_test.sh`, Windows에서는 Go를 설치한 뒤 `./tests/install_test.ps1`로 실행합니다.

## 문제 해결

### `go: command not found`

검증된 mise toolchain으로 Go를 실행하세요:

```bash
mise x go@1.24 -- go version
```

### `redmine: command not found`

릴리스 설치 스크립트를 사용했다면 현재 셸의 `PATH`를 갱신하세요:

```bash
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
