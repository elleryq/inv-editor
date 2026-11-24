# Debian 套件打包規劃

> 對應 issue：[#3 Release 的 assets 包含 debian package](https://github.com/elleryq/inv-editor/issues/3)
> 狀態：規劃完成，待實作

## 目標

推 `v*` tag 發版時，GitHub Release 除了現有的 tar.gz / zip，也附上可以直接安裝的 `.deb`：

```bash
curl -LO https://github.com/elleryq/inv-editor/releases/download/v1.4.0/inv-editor_1.4.0_amd64.deb
sudo apt install ./inv-editor_1.4.0_amd64.deb
```

**不在這次範圍**：自架 APT repository（`apt update` 自動更新）、套件簽章、送進 Debian/Ubuntu 官方套件庫、rpm/apk。這些放在[後續擴充](#後續擴充)。

## 現況分析

| 項目 | 現況 | 對打包的影響 |
|---|---|---|
| 執行檔 | 單一執行檔 `inv-editor`，`CGO_ENABLED=0` 靜態編譯 | 不需要宣告 `Depends`，同一個 .deb 可以裝在任何 Debian/Ubuntu 版本 |
| 靜態資源 | `internal/web` 用 `//go:embed templates static` 把模板和 CSS 包進執行檔 | 不需要安裝 `/usr/share/inv-editor` 之類的資料目錄 |
| 設定檔 | 沒有，全部透過參數指定 | 不需要 `conffiles` |
| 背景服務 | `serve` 是使用者手動啟動的前景程式 | 第一版不附 systemd unit |
| 版本資訊 | 程式沒有 `--version`，執行檔裡也沒有嵌入版本號 | 建議一起補上，見[前置工作](#前置工作) |
| 發版流程 | `release.yml` 用 matrix 編 5 個平台，上傳 artifact 後在 `release` job 建立 Release 並產生 `SHA256SUMS.txt` | .deb 只要放進同一個 `dist/`，checksum 和上傳都會自動涵蓋 |
| 授權 | MIT | 放進 `/usr/share/doc/inv-editor/copyright` |

結論：專案的條件很適合打包，主要的工作是把 .deb 產生步驟接進現有的 release workflow。

## 工具選擇

| 方案 | 優點 | 缺點 |
|---|---|---|
| **nfpm**（建議） | 宣告式 YAML 設定；不需要 Debian 環境就能交叉產生 amd64/arm64；之後要出 rpm、apk 只要改一個參數；GoReleaser 底層就是用它 | 多一個建置工具，要固定版本 |
| `dpkg-deb --build` 手寫 | ubuntu runner 內建，沒有額外依賴 | 要自己組 `DEBIAN/control`、目錄結構、權限、`md5sums`；要擴充成 rpm 時得重寫 |
| 整個改用 GoReleaser | 一次處理 build、打包、checksum、Release | 要重寫整個 `release.yml`，改動範圍遠大於這個 issue |
| debhelper / `dh-golang` | Debian 官方正規做法 | 需要 vendoring 和完整的 `debian/` 目錄，適合送官方套件庫，不適合 GitHub Release |

**建議用 nfpm**，固定版本為目前最新的 `v2.49.0`。

## 套件規格

| 欄位 | 值 | 說明 |
|---|---|---|
| Package | `inv-editor` | 跟執行檔同名 |
| Version | tag 去掉 `v`，例如 `v1.4.0` → `1.4.0` | 預發版 `v1.4.0-rc1` 要轉成 `1.4.0~rc1`，`~` 才會讓它排在正式版前面 |
| Architecture | `amd64`、`arm64` | 對應現有的 linux matrix；GOARCH 名稱和 Debian 一致，不用轉換 |
| Section | `admin` | 系統管理工具（Ansible 相關） |
| Priority | `optional` | |
| Maintainer | `Ellery Tsai <22297+elleryq@users.noreply.github.com>` | 用 GitHub noreply 位址，不在套件裡公開個人信箱 |
| Homepage | `https://github.com/elleryq/inv-editor` | |
| Depends | 無 | 靜態編譯 |
| Suggests | `ansible` | 不強制，但提示使用者這是 Ansible 周邊工具 |
| Description | `Interactive editor for Ansible inventory files` 加上 TUI / web 兩種模式的說明 | |

**檔名**：依 Debian 慣例 `inv-editor_<version>_<arch>.deb`，例如 `inv-editor_1.4.0_amd64.deb`。跟現有的 `inv-editor-v1.4.0-linux-amd64.tar.gz` 命名不同，但這是 apt 使用者熟悉的格式。

**安裝內容**：

| 路徑 | 來源 | 權限 |
|---|---|---|
| `/usr/bin/inv-editor` | 編好的執行檔 | 0755 |
| `/usr/share/doc/inv-editor/copyright` | `LICENSE` | 0644 |
| `/usr/share/doc/inv-editor/README.md` | `README.md` | 0644 |

## 實作步驟

### 前置工作

**加上 `--version`**：.deb 有版本號，但使用者裝完之後沒辦法從程式確認版本，回報問題時也不方便。

- `cmd/inv-editor/main.go` 加 `var version = "dev"`，並處理 `version` / `--version` 子命令。
- `release.yml` 的 build 步驟加 `-X main.version=${VERSION}`。
- `go install` 或自己 build 時沒有 ldflags，可以改用 `runtime/debug.ReadBuildInfo()` 讀取 module 版本當備案。

### 1. 新增 `nfpm.yaml`

放在 repo 根目錄，版本和架構由環境變數帶入：

```yaml
name: inv-editor
arch: ${DEB_ARCH}
platform: linux
version: ${DEB_VERSION}
version_schema: semver
section: admin
priority: optional
maintainer: "Ellery Tsai <22297+elleryq@users.noreply.github.com>"
description: |
  Interactive editor for Ansible inventory files.
  Provides a keyboard-driven terminal UI and an optional web interface
  for managing groups, hosts and variables in INI or YAML inventories.
homepage: https://github.com/elleryq/inv-editor
license: MIT
suggests:
  - ansible
contents:
  - src: ./inv-editor
    dst: /usr/bin/inv-editor
    file_info:
      mode: 0755
  - src: ./LICENSE
    dst: /usr/share/doc/inv-editor/copyright
    file_info:
      mode: 0644
  - src: ./README.md
    dst: /usr/share/doc/inv-editor/README.md
    file_info:
      mode: 0644
```

`version_schema: semver` 會讓 nfpm 把 `1.4.0-rc1` 拆成 `1.4.0~rc1`，處理預發版的排序。

### 2. 修改 `release.yml`

在 `build` job 的 linux matrix 項目加一個打包步驟。產生的 .deb 跟 tar.gz 一起上傳成同一個 artifact，`release` job 完全不用改：

```yaml
      - name: Build .deb
        if: matrix.goos == 'linux'
        env:
          DEB_ARCH: ${{ matrix.goarch }}
          NFPM_VERSION: v2.49.0
        run: |
          export DEB_VERSION="${GITHUB_REF_NAME#v}"
          go install github.com/goreleaser/nfpm/v2/cmd/nfpm@"${NFPM_VERSION}"
          nfpm package --packager deb --config nfpm.yaml --target .
```

另外要修改：

- `Package archive` 步驟把 .deb 檔名也寫進 `$GITHUB_OUTPUT`，或者 `upload-artifact` 的 `path` 改成多行，同時列 tar.gz 和 `*.deb`。
- `GITHUB_REF_NAME` 透過 `env` 讀取，不直接把 `${{ github.ref_name }}` 寫進 `run:`。

### 3. 在 CI 驗證打包

`ci.yml` 加一個 job，在 PR 上就先檢查 .deb 的內容，避免發版當下才發現問題：

1. 用 `DEB_VERSION=0.0.0~ci` 產生 amd64 的 .deb。
2. `dpkg-deb --info` 和 `dpkg-deb --contents` 檢查欄位和檔案清單。
3. `lintian` 檢查（警告先不擋，只擋 error）。
4. `sudo apt install ./inv-editor_*.deb`，接著跑 `inv-editor --help` 和 `inv-editor --version`，最後 `sudo apt remove inv-editor`。

arm64 可以用 GitHub 的 `ubuntu-24.04-arm` runner 做同樣的安裝測試；如果覺得成本太高，至少用 `dpkg-deb --info` 確認 Architecture 欄位正確。

### 4. 更新文件

- README 的 Installation 加上 .deb 安裝方式。
- 順便修正 README 現有的 `go install github.com/elleryq/inv-editor@latest`：`main` 在 `cmd/inv-editor` 底下，正確的指令是 `go install github.com/elleryq/inv-editor/cmd/inv-editor@latest`。

### 5. 發版驗證

發 `v1.4.0`（新增 `--version` 功能，所以升 minor 版本）之後確認：

- Release assets 有 `inv-editor_1.4.0_amd64.deb` 和 `inv-editor_1.4.0_arm64.deb`
- `SHA256SUMS.txt` 有這兩個檔案，`sha256sum -c` 通過
- 在乾淨的 Debian 和 Ubuntu 容器各安裝一次，`inv-editor --version` 顯示 `1.4.0`

## 建議的 PR 拆分

| PR | 內容 | 依賴 |
|---|---|---|
| 1 | 加 `--version`，`release.yml` 嵌入版本號 | 無 |
| 2 | 新增 `nfpm.yaml`、`release.yml` 打包步驟、`ci.yml` 驗證 job | PR 1 |
| 3 | README 安裝說明與 `go install` 路徑修正 | PR 2 |

PR 2 和 PR 3 可以合併成一個。

## 已決定事項

1. **Maintainer 的 email**：用 GitHub noreply 位址 `22297+elleryq@users.noreply.github.com`。Debian 的 `Maintainer` 欄位會公開在套件裡，不放個人信箱。
2. **不支援 armhf**：只出 `amd64` 和 `arm64`，跟現有的 linux matrix 一致。
3. **版本號**：第一個附 .deb 的版本是 `v1.4.0`。

## 後續擴充

- **APT repository**：用 GitHub Pages 加 `reprepro` 或 `aptly` 架設，需要 GPG 簽章金鑰和保管流程，使用者才能 `apt update` 自動更新。
- **rpm / apk**：nfpm 的 `--packager rpm` / `apk`，用同一份 `nfpm.yaml`。
- **man page**：從 `helpText` 產生 `inv-editor.1`，安裝到 `/usr/share/man/man1/`。
- **shell completion**：目前子命令很少，需求不高。
- **systemd unit**：如果有人要把 `serve --readonly` 當常駐服務，可以附一個停用狀態的範本 unit。
