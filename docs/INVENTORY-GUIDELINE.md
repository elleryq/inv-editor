# inventory 編輯規則

## 結構概覽

```
all:
  hosts:          ← 所有主機及其 ansible_host 集中定義於此
  children:       ← 各分組只列主機名稱，不重複寫 ansible_host
    group_name:
      hosts:
        hostname:
```

**核心原則：`ansible_host` 只在 `all.hosts` 定義一次。群組內不寫 IP。**

---

## 新增主機

### 有 IP 的主機

在 `all.hosts` 的 `# ---- Hosts with IP ----` 區段新增，依 IP 排序：

```yaml
all:
  hosts:
    # ---- Hosts with IP ----
    new-server:
      ansible_host: 10.11.21.99
```

### 沒有 IP 的主機（VM 存在但 IP 未知或不可達）

在 `all.hosts` 的 `# ---- Hosts without IP ----` 區段新增，只寫主機名：

```yaml
    # ---- Hosts without IP ----
    new-server-no-ip:
```

---

## 將主機加入群組

主機必須已在 `all.hosts` 定義。群組內只寫主機名，不加 `ansible_host`：

```yaml
  children:
    LAN60:
      hosts:
        new-server:      # ← 只有名稱，無 IP
```

---

## 修改主機 IP

只需修改 `all.hosts` 中的 `ansible_host`，所有引用該主機的群組自動生效：

```yaml
    some-host:
      ansible_host: 10.11.21.200   # ← 改這裡就好
```

---

## 移除主機

需同時從兩處刪除：
1. `all.hosts` 的主機定義
2. 所有有列出該主機名的群組

---

## 主機名稱需加引號的情況

以下情況主機名或群組名必須用雙引號 `"..."` 包住：

| 情況 | 範例 |
|------|------|
| 數字開頭 | `"2520-OCP_DNS"`, `"10.11.18.231"` |
| 含 `%` | `"chester-lab%2fOCPbation"` |
| 含 `&` | `"Mason.Tan-Bitbucket&Jenkins&Draw.io"` |
| 含 `/` | `"CentOS_4/5/6/7_64_bit"` |
| 含 `.`（群組名） | `"oy-BIG-IQ-8.2.0.1-0.0.97"` |

主機名含 `-`、`_`、英數字則**不需要**引號。

---

## 群組說明

| 群組 | 用途 |
|------|------|
| `with_ip` | 所有有 IP 的主機（維護用分類） |
| `without_ip` | 所有沒有 IP 的主機 |
| `ouyang` | 歐陽環境（含 ouyang 相關 VM） |
| `b3` | b3 網段主機 |
| `LAN60` | `10.11.60.x` 網段主機 |
| `ocp` | OCP cluster（目前全部停用，保留結構） |
| `10.11.18.231` 等 | 依 ESXi 主機分組 |
| `Unknown_OS` / `Red_Hat_*` 等 | 依作業系統分組（由 govc 產生） |
| `no_exporter` | 確認無 node_exporter 的 IP 清單 |
| `no_exporter_bad` | 無 node_exporter 且狀態異常的 IP |

> `no_exporter` / `no_exporter_bad` 的成員是**純 IP 字串**（非主機名），屬特例。

---

## 注意事項

- **不要在群組內寫 `ansible_host`**，違反一元管理原則
- 同一主機可同時屬於多個群組（OS 群 + ESXi 群 + 邏輯群並存）
- `ocp` 群組內容全部停用中，新增 OCP 主機請先確認網路連通性
- 原 `vc8.ini` 已廢棄，請以 `vc8.yml` 為主
