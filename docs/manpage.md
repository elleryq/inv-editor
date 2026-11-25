# man page 規劃

> 對應 issue：[#10 加上 man page（inv-editor(1)）](https://github.com/elleryq/inv-editor/issues/10)
> 狀態：已實作，隨 RPM（#4）之後的版本發佈；發版後確認 `man inv-editor`

## 目標

安裝 .deb 或 RPM（[#4](https://github.com/elleryq/inv-editor/issues/4)）之後，可以用 `man inv-editor` 查看完整說明，lintian 也不再出現 `no-manual-page`。

**不在這次範圍**：其他語言的 man page、shell completion、把 man page 放進 tar.gz / zip（見[已決定事項](#已決定事項)）。

## 現況分析

目前 CLI 說明有兩份，而且已經跟程式不一致：

| 來源 | 問題 |
|---|---|
| `cmd/inv-editor/main.go` 的 `helpText`（`--help`） | 還列著 `M`（搬移群組），但程式已經沒有處理這個按鍵；沒有 `i`（匯入）、`/`（搜尋）、`space`（多選）、`p`（貼上）、`Esc`；`c` / `m` 仍是舊的「直接複製/搬移到另一個群組」說明，現在是「標記後用 `p` 貼上」 |
| README 的 TUI Controls | 比 `--help` 完整，但沒有 `/`（搜尋）和 `Shift+←` / `Shift+→`（水平捲動） |
| `docs/SPEC.md` | 沒有 TUI 搜尋 |

按鍵實際的處理在 `internal/tui/keys.go`。加上 man page 會變成第三份，所以這次要一起處理同步問題。

## 工具選擇

| 方案 | 優點 | 缺點 |
|---|---|---|
| **go-md2man**（建議） | 用 Markdown 寫；Go 1.24 起可以用 go.mod 的 `tool` 指令固定版本，`go tool go-md2man` 執行，不用另外安裝；只依賴 `russross/blackfriday/v2`；Docker、Podman 都用它產生 man page | go.mod 會多一個 tool 依賴 |
| 直接手寫 roff | 不需要任何工具 | roff 語法難維護，貢獻者不熟 |
| pandoc | 功能最完整 | CI 和 release runner 都要額外 `apt-get install`，套件很大 |
| 從 Go 程式碼產生 | 理論上可以跟程式同步 | 專案沒有用 cobra 之類的 CLI 框架，要自己寫產生器，按鍵說明也不在結構化資料裡 |

go-md2man 最新版是 v2.0.7，`go.mod` 宣告 `go 1.12`，不會碰到 nfpm 那種工具鏈版本太新的問題。

## 設計

### 檔案

| 檔案 | 說明 |
|---|---|
| `docs/man/inv-editor.1.md` | man page 原始檔，進版控 |
| `build/pkg/inv-editor.1.gz` | 打包時產生，不進版控（`build/` 已被忽略） |

產生出來的 roff 檔不進版控，避免原始檔和產物不同步。

### 內容

| 段落 | 內容 |
|---|---|
| NAME | `inv-editor - interactive editor for Ansible inventory files` |
| SYNOPSIS | `inv-editor` *file*、`inv-editor serve` [*options*] *file*（見[實作時發現的問題](#實作時發現的問題)）、`inv-editor help`、`inv-editor version` |
| DESCRIPTION | 兩種模式（TUI、web）；檔案不存在時從空 inventory 開始 |
| COMMANDS | 無子命令（TUI）、`serve`、`help`、`version` |
| OPTIONS | `serve` 的 `--host`、`--port`、`--readonly` |
| TUI KEYS | 以 `internal/tui/keys.go` 為準的完整按鍵列表，包含搜尋、多選、複製/搬移/貼上、匯入、匯出 |
| FILES | 支援的格式和副檔名（INI：`.ini`、`.cfg`；YAML：`.yml`、`.yaml`），`/download` 一律回傳 YAML |
| EXAMPLES | 取自現有 `--help` 和 README |
| SECURITY | `serve` 預設只聽 `127.0.0.1`；改成 `0.0.0.0` 前要考慮沒有認證，建議搭配 `--readonly` |
| SEE ALSO | `ansible-inventory(1)`、`ansible(1)`、專案網址 |
| BUGS | GitHub issues 網址 |

標題行帶版本號和日期。go-md2man 把第一個 `===` 標題轉成 roff 的 `.TH`，原始檔寫成：

```markdown
inv-editor 1 "@DATE@" "inv-editor @VERSION@" "User Commands"
=============================================================
```

`@VERSION@`、`@DATE@` 在打包時用 tag 和 release commit 日期替換，跟 Debian changelog 的做法一致，重新 build 會得到相同內容。

### 安裝位置

`.deb` 和 RPM 都安裝 `/usr/share/man/man1/inv-editor.1.gz`（`gzip -9n`，權限 0644）。RPM 通常由 `rpmbuild` 的 brp 腳本自動壓縮 man page，nfpm 不會，所以兩種套件都直接放壓縮好的檔案。

### 同步問題

1. **以 man page 為完整參考**：`--help` 保持簡短，只列用法、子命令、`serve` 參數和常用按鍵，最後提示 `man inv-editor` 看完整說明。README 的 TUI Controls 照 man page 補齊。
2. **這次一起修正**：`--help` 移除不存在的 `M`、補上缺的按鍵；README 和 SPEC 補上搜尋與水平捲動。
3. **測試防止遺漏**：在 `cmd/inv-editor` 加一個測試，讀取 `docs/man/inv-editor.1.md`，確認每個子命令和 `serve` 的每個 flag 都有出現。flag 從 `flag.FlagSet` 取得，之後新增 flag 卻忘了寫進 man page，測試就會失敗。
   - 按鍵分散在 `keys.go` 的多個 `switch`，沒有結構化的清單，所以不做自動檢查，改在 PR 說明提醒人工核對。之後如果要做，得先把按鍵整理成一張表。

## 實作步驟

1. **加 tool 依賴**：`go get -tool github.com/cpuguy83/go-md2man/v2@v2.0.7`，確認 `go mod tidy` 和 CI 的 `go mod tidy -diff` 都通過。
2. **撰寫 `docs/man/inv-editor.1.md`**，並修正 `--help`、README、SPEC。
3. **同步測試**：加上上面第 3 點的測試。
4. **打包**：打包腳本先產生 `build/pkg/inv-editor.1.gz`（替換版本和日期 → `go tool go-md2man` → `gzip -9n`），nfpm 設定加上 contents（deb 和 RPM 各一筆，見下方「實作時發現的問題」）。
5. **CI 檢查**：
   - `groff -man -Tutf8 -ww -z`，有任何輸出就失敗（groff 遇到 warning 仍然回傳 0，`man --warnings` 也會把整頁輸出混在一起，不適合拿來判斷）
   - `packages` job 的 lintian 不再出現 `no-manual-page`，並把 lintian 改成 `--fail-on warning`，之後有新 warning 也會被擋下
   - 安裝後確認 `man -w inv-editor` 找得到 man page
6. **更新文件**：`docs/packaging.md` 的後續擴充移除 man page，改為已完成。

依賴 #4 的打包調整，所以在 #4 之後實作，PR 疊在 #4 的分支上。

## 實作時發現的問題

- **`serve` 放在檔案後面的 flag 會被忽略**：Go 的 `flag` 套件在第一個非 flag 參數停止解析，`inv-editor serve vc8.yaml --readonly` 實際上會以可寫入模式跑在 8080。原本 `--help` 和 README 的範例都是這種寫法。已在 #14 修正，現在 flag 放在檔案前後都可以；man page、`--help` 和 README 的範例統一寫成 flag 在前，並註明 1.5.0 之前的版本會忽略放在後面的 flag。
- **nfpm 的 deb 不收 `type: doc`**：man page 要依格式各寫一筆 contents，RPM 那筆才標 `type: doc`。
- **Ubuntu 容器 image 無法測 `man`**：minimized image 把 `man` 換成提示訊息，本機改用 `debian:stable`（非 slim）測試；GitHub 的 ubuntu runner 是完整系統，不受影響。

## 已決定事項

1. **工具**：go-md2man v2.0.7，用 go.mod 的 `tool` 指令固定版本。
2. **產物不進版控**：只進版控原始 Markdown，打包時產生。
3. **tar.gz / zip 不放 man page**：維持只有執行檔。macOS 和 Linux 使用者手動安裝時也很少另外放 man page；需要的話之後再加。
4. **完整參考以 man page 為準**：`--help` 保持精簡。

## 驗證

- [x] `man -l` 顯示正常，`groff -ww` 沒有 warning
- [x] lintian 沒有任何 warning（CI 改成 `--fail-on warning`）；rpmlint 沒有 `no-manual-page-for-binary`
- [x] 安裝 .deb（`debian:stable`）和 RPM（`almalinux:9`）後 `man inv-editor` 可以開啟，標題行版本正確
- [ ] 發版後從 Release 下載的套件確認一次
- [x] 同步測試會在漏寫 flag 時失敗（暫時拿掉 man page 裡的 `--readonly` 驗證）；拿掉 COMMANDS 裡的 `version` 也會失敗
