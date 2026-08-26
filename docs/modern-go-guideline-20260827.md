# Modern Go Guidelines 檢視報告 (2026-08-27)

依據 [JetBrains Modern Go Guidelines](https://github.com/JetBrains/go-modern-guidelines) CLI，針對本專案（`go.mod` 宣告 `go 1.23.0`）解析出的適用規則，檢視 `internal/`、`cmd/` 下所有 Go 原始檔並套用修正。

## 套用的修改

| 規則 | 說明 | 位置 |
| --- | --- | --- |
| `slices_sort` | 以 `slices.Sort` 取代 `sort.Strings` | `internal/inventory/ini.go` (`sortedKeys`)、`internal/inventory/yaml.go` (`sortedStringKeys`)、`internal/tui/model.go` (`sortedMapKeys`)、`internal/web/handlers.go` (`buildPageData` 內兩處 var-key 排序) |
| `slices_contains` | 以 `slices.Contains` 取代手寫搜尋迴圈 | `internal/inventory/ini.go`：刪除自訂的 `containsStr`，改用 `slices.Contains` |
| `slices_clone` | 以 `slices.Clone` 取代 `append([]T(nil), x...)` | `internal/inventory/model.go` (`CopyGroupDeep` 內複製 `children`、`hosts`) |
| `maps_clone` | 以 `maps.Clone` 取代手動 `make` + 迴圈複製 map | `internal/inventory/model.go` (`CopyGroupDeep` 內複製 `vars`；`MergeFrom` 內建立新 host 的 `Vars`) |
| `min_max` | 內建 `min`/`max`（Go 1.21+）已足夠，刪除重複定義 | `internal/tui/model.go`：移除自訂的 `func max(a, b int) int` 與 `func min(a, b int) int`，呼叫端維持不變（改用內建版本） |
| `range_over_int` | 以 `for i := range n` 取代 `for i := 0; i < n; i++` | `internal/inventory/ini.go` (`splitVars`) |

修改原則：僅套用「等價替換」，不變更任何行為或邏輯。

## 檢視過但未變更的部分（已符合現代寫法）

- `internal/web/server.go`：已使用 method-aware `http.ServeMux` pattern（如 `"GET /api/groups"`）與 `http.FileServerFS`（`http_servemux_patterns`）。
- 全專案已使用 `strings.Cut` / `strings.CutSuffix`（`strings_cut_prefix_suffix` / `strings_cut`）。
- `internal/inventory/model.go` 多處已使用 `slices.DeleteFunc`、`slices.Contains`。
- 未發現 `interface{}`（均可用 `any`，本專案本就未使用 `interface{}`）。
- 未發現需要 `errors.Is`、`errors.Join`、atomic 型別、`sync.OnceFunc`/`OnceValue`、`context.AfterFunc`/`WithCancelCause`、`time.Since`/`time.Until`、`clear()`、`maps.DeleteFunc` 等規則適用的程式碼（專案未使用計時器、原子操作、context 取消鏈或需要清空整個 map/slice 的情境）。

## 驗證

- `go build ./...` — 通過
- `go vet ./...` — 通過
- `go test ./...` — 通過（`internal/inventory` 測試全過；`tui`/`web`/`cmd` 無測試檔案）

## 未處理的既有問題（超出本次範圍）

`gofmt -l` 顯示以下檔案有既存的欄位/常數對齊問題，這些問題在本次修改前即已存在，且與 modern-go-guidelines 無關，故未一併調整：

- `internal/inventory/ini.go`
- `internal/tui/model.go`
- `internal/tui/styles.go`
- `internal/web/handlers.go`
