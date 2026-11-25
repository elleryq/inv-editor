# Debian 套件打包規劃

> 對應 issue：[#3 Release 的 assets 包含 debian package](https://github.com/elleryq/inv-editor/issues/3)
> 狀態：實作中（PR 1、PR 2 已完成，PR 3 進行中），待 v1.4.0 發版驗證

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

**採用 nfpm**，固定版本 `v2.49.0`。

nfpm v2.49.0 本身需要 Go ≥ 1.27.4 才能編譯，比專案用的 Go 1.26 新，所以不用 `go install`，改下載官方編好的 binary。下載後用寫死在 [install-nfpm.sh](../packaging/deb/install-nfpm.sh) 裡的 SHA-256 驗證，檔案被換掉時 CI 會直接失敗。要升級 nfpm 時，同時更新版本號和兩個 hash。

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
| `/usr/share/doc/inv-editor/changelog.gz` | 打包時產生 | 0644 |
| `/usr/share/doc/inv-editor/README.md` | `README.md` | 0644 |
| `/usr/share/lintian/overrides/inv-editor` | `packaging/deb/lintian-overrides` | 0644 |

## 實作步驟

### 前置工作（已完成，PR #6）

加上 `inv-editor version` / `--version`。版本來源依序為 ldflags 嵌入的 tag、Go 工具鏈記錄的 module 版本、`dev`；`release.yml` 在 build 時用 `-X main.version=<tag>` 嵌入。

### 1. 打包檔案（`packaging/deb/`）

根目錄的 `.gitignore` 忽略所有 `*.yaml`（避免誤提交使用者的 inventory），所以打包設定集中放在 `packaging/deb/`，並在 `.gitignore` 加上 `!packaging/**/*.yaml` 例外。

| 檔案 | 用途 |
|---|---|
| [nfpm.yaml](../packaging/deb/nfpm.yaml) | 套件欄位和安裝內容；`DEB_ARCH`、`DEB_VERSION` 由環境變數帶入。`version_schema: semver` 讓 nfpm 把 `1.4.0-rc1` 轉成 `1.4.0~rc1` |
| [build.sh](../packaging/deb/build.sh) | `build.sh <binary> <tag> <arch> <out-dir>`：檢查 tag 格式與架構、把檔案放到 `build/deb/`、產生 changelog、呼叫 nfpm。CI 和 release 共用這支腳本 |
| [install-nfpm.sh](../packaging/deb/install-nfpm.sh) | 下載固定版本的 nfpm binary 並驗證 SHA-256 |
| [lintian-overrides](../packaging/deb/lintian-overrides) | 說明靜態編譯是刻意的 |

**lintian 處理**：第一次跑 lintian 出現兩個 error 和一個 warning。

- `no-changelog`：Debian policy 要求 native package 附 `changelog.gz`。`build.sh` 產生一份最小的 Debian changelog，日期用 release commit 的時間，重新 build 也會得到相同內容。
- `statically-linked-binary`：Go 用 `CGO_ENABLED=0` 刻意靜態編譯。比照 Debian 的 Go 套件，附 lintian override。
- `no-manual-page`（warning）：先保留，man page 列在[後續擴充](#後續擴充)。

### 2. 修改 `release.yml`

linux matrix 項目在 build 之後多兩個步驟：`install-nfpm.sh` 安裝 nfpm，`build.sh` 產生 .deb。tag 透過 `env` 傳入，不直接把 `${{ github.ref_name }}` 寫進 `run:`。

.deb 要輸出到工作目錄根層，跟 tar.gz 放在一起。`upload-artifact` 收到多個路徑時，會以它們的共同上層目錄當根目錄；如果 .deb 放在子目錄，`release` job 的 `dist/` 裡會多一層資料夾，`sha256sum *` 和 `gh release create dist/*` 都會出錯。放在根層的話，`release` job 不用改，`SHA256SUMS.txt` 也會自動包含 .deb。

### 3. 在 CI 驗證打包

`ci.yml` 新增 `deb` job，amd64 跑在 `ubuntu-latest`，arm64 跑在 `ubuntu-24.04-arm`，兩種架構都實際安裝：

1. 用 tag `v0.0.0-ci` build 執行檔並產生 .deb（套件版本 `0.0.0~ci`）。
2. `dpkg-deb --info` / `--contents`，並確認 `Architecture` 和 `Version` 欄位。
3. `lintian --fail-on error`（warning 不擋）。
4. `apt-get install` 安裝，確認 `inv-editor --version` 輸出 `inv-editor 0.0.0-ci`、`--help` 正常，再 `apt-get remove` 並確認執行檔已移除。

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
| 1 | 加 `--version`，`release.yml` 嵌入版本號（#6，已合併） | 無 |
| 2 | 新增 `packaging/deb/`、`release.yml` 打包步驟、`ci.yml` 驗證 job（#7，已合併） | PR 1 |
| 3 | README 安裝說明與 `go install` 路徑修正 | PR 2 |

## 已決定事項

1. **Maintainer 的 email**：用 GitHub noreply 位址 `22297+elleryq@users.noreply.github.com`。Debian 的 `Maintainer` 欄位會公開在套件裡，不放個人信箱。
2. **不支援 armhf**：只出 `amd64` 和 `arm64`，跟現有的 linux matrix 一致。
3. **版本號**：第一個附 .deb 的版本是 `v1.4.0`。

## 後續擴充

- **APT repository**：用 GitHub Pages 加 `reprepro` 或 `aptly` 架設，需要 GPG 簽章金鑰和保管流程，使用者才能 `apt update` 自動更新。
- **rpm / apk**：nfpm 的 `--packager rpm` / `apk`，用同一份 `nfpm.yaml`（changelog 和 lintian override 只適用 deb，要另外處理）。
- **man page**：從 `helpText` 產生 `inv-editor.1`，安裝到 `/usr/share/man/man1/`。
- **shell completion**：目前子命令很少，需求不高。
- **systemd unit**：如果有人要把 `serve --readonly` 當常駐服務，可以附一個停用狀態的範本 unit。
