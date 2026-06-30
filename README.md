# inv-editor

A terminal-based interactive editor for Ansible inventory files.

Built with Go + [Bubble Tea](https://github.com/charmbracelet/bubbletea).

## Features

- Create inventory from scratch or open an existing file
- Support INI and YAML Ansible inventory formats
- Manage groups, hosts, and variables with a keyboard-driven TUI
- Multi-select hosts and copy/move them (or whole groups) between groups via a clipboard (mark with `c`/`m`, paste with `p`)
- Import another inventory file and merge it into the one you're editing
- Export to any supported format
- Single static binary, no dependencies

## Installation

```bash
# Build from source
git clone https://github.com/elleryq/inv-editor
cd inv-editor
mkdir build
go build -o build/inv-editor ./cmd/inv-editor

# Or install directly
go install github.com/elleryq/inv-editor@latest
```

## Usage

```bash
# Terminal UI — open or create an inventory file
inv-editor vc8.ini
inv-editor production.yaml

# If the file doesn't exist, starts with an empty inventory
inv-editor new-inventory.ini

# Web interface
inv-editor serve vc8.yaml                            # default: 127.0.0.1:8080
inv-editor serve vc8.yaml --port 9090
inv-editor serve vc8.yaml --host 127.0.0.1           # local only
inv-editor serve vc8.yaml --host 0.0.0.0 --readonly  # read-only mode
```

## TUI Controls

| Key | Action |
|-----|--------|
| `Tab` / `Shift+Tab` | Cycle between panels (Groups → Hosts → Variables) |
| `G` / `H` / `V` | Jump directly to Groups / Hosts / Variables panel |
| `↑` `↓` or `j` `k` | Navigate items in current panel |
| `n` | New item — group, host (hostname + optional IP, stored as `ansible_host`), or variable |
| `e` / `Enter` | Edit selected item |
| `d` / `Delete` | Delete selected item |
| `space` | Toggle multi-select for the host under the cursor (Hosts panel) |
| `c` | Mark the selected host(s) — or current group — to **copy** |
| `m` | Mark the selected host(s) — or current group — to **move** |
| `p` | Paste the marked host(s)/group into the group currently selected in the Groups panel |
| `Esc` | Clear a pending copy/move clipboard |
| `v` | Open variables for selected group or host |
| `s` | Save to original file |
| `x` | Export to a different format/path |
| `i` | Import another inventory file and merge it into the current one |
| `q` | Quit (Save & Quit / Discard / Cancel if unsaved changes) |
| `?` | Toggle help overlay |

### Copy / Move workflow

Both the Hosts panel and the Groups panel use the same clipboard flow:

1. Press `c` (copy) or `m` (move) on a host (or multi-selected hosts) or on a group.
2. Switch to the Groups panel (`Tab`/`G`) and select the destination group.
3. Press `p` to paste.

- Copying a host adds it to the target group without removing it from the source.
- Moving a host removes it from the source group and adds it to the target.
- Copying a group deep-copies it (subgroups, member hosts, and vars) under the target as a new group — you'll be prompted for a new name to avoid collisions.
- Moving a group reparents it under the target group.

### Import

`i` opens a file path prompt. The selected inventory file is parsed and merged into the one currently open:

- Groups and hosts are matched **by name** — same-named items are merged, not duplicated.
- On a variable conflict, the currently open inventory's value always wins; new variables from the imported file are added.
- Host group-membership is unioned (a host keeps all the groups it belonged to in either file).

## Screen Layout

```
┌─────────────────────────────────────────────────────────────┐
│  inv-editor: vc8.ini  [modified]           Press ? for help │
├───────────────────────┬─────────────────────────────────────┤
│  GROUPS          [G]  │  HOSTS (webservers)            [H]  │
│                       │                                     │
│   all                 │   [ ] web01.example.com              │
│ > webservers          │ > [x] web02.example.com              │
│   dbservers           │   [ ] web03.example.com              │
│   [+ New Group]       │   [+ New Host]                       │
├───────────────────────┴─────────────────────────────────────┤
│  VARIABLES  (web02.example.com)                        [V]  │
│                                                             │
│   ansible_user  = ubuntu                                    │
│   ansible_port  = 22                                        │
│   [+ New Variable]                                          │
├─────────────────────────────────────────────────────────────┤
│  Tab next panel │ n new │ e edit │ d del │ s save │ q quit  │
└─────────────────────────────────────────────────────────────┘
```

## Supported Formats

### Read
- `*.ini`, `*.cfg` — Ansible INI inventory
- `*.yml`, `*.yaml` — Ansible YAML inventory

### Write / Export
- INI format
- YAML format

## INI vs YAML：格式選擇說明

### 為什麼建議使用 YAML？

YAML 格式天生支援「單一來源」（single source of truth）的管理原則：

```yaml
all:
  hosts:
    server1:
      ansible_host: 10.11.21.1   # ← ansible_host 只定義在這裡
  children:
    webservers:
      hosts:
        server1: null             # ← 子群組只列名稱，不重複寫 IP
```

**inv-editor 的 YAML writer 遵循此原則**：存檔時自動將所有 host 的變數集中寫入 `all.hosts`，子群組只保留主機名稱。

### INI 格式的限制

INI 格式沒有獨立的 `all.hosts` 區段，host 變數只能寫在 section 裡：

```ini
[webservers]
server1 ansible_host=10.11.21.1   # 只能寫在 group section 內

[monitored]
server1                            # 省略 vars，靠 Ansible 合併
```

若同一台主機出現在多個 group，`ansible_host` 必須寫在其中一個 group（通常第一個），其餘省略。這在閱讀上不直觀，且沒有結構上的「集中位置」可對應 YAML 的 `all.hosts`。**INI 格式本身無法保證 single source of truth。**

### 建議工作流程

如果現有的是 INI 格式，建議一次性遷移到 YAML：

```
vc8.ini ──(inv-editor 開啟)──▶ 按 x 匯出 YAML ──▶ vc8.yml
                                                       ↑
                                               之後只編輯此檔案
```

## Web Interface

Start the web server to browse and edit your inventory in a browser:

```bash
inv-editor serve vc8.yaml
```

Then open `http://localhost:8080`.

| Feature | Description |
|---------|-------------|
| Three-panel layout | Groups tree (left) · Hosts (right top) · Variables (right bottom) |
| Group tree | Expand/collapse subgroups; click to select |
| Host management | Add, rename, delete, move, or copy hosts |
| Variable editor | Inline add/edit/delete for group or host vars |
| Save | Writes back to the original file in its original format |
| Download YAML | `GET /download` — always returns YAML regardless of source format |
| Read-only mode | `--readonly` flag hides all edit controls; mutations return 403 |

> **Security note**: When listening on `0.0.0.0`, anyone on the network can access the editor. Use `--readonly` or `--host 127.0.0.1` if this is a concern.

## Project Structure

```
inv-editor/
├── cmd/inv-editor/     # Entry point (TUI + serve subcommand)
├── internal/
│   ├── inventory/      # Data model + INI/YAML parser & writer
│   ├── tui/            # Bubble Tea terminal UI
│   └── web/            # HTTP server, handlers, templates, CSS
│       ├── static/     # Embedded CSS
│       └── templates/  # Embedded HTML templates
├── docs/
│   ├── SPEC.md         # Full functional specification
│   └── INVENTORY-GUIDELINE.md
└── README.md
```

## Ansible Inventory Notes

- `all` group is always present (Ansible built-in root group)
- Subgroups (group-of-groups) are supported; navigate with `→`/`←` in the Groups panel
- Variables panel shows host or group vars; press `v` on a group or host to open it
- `group_vars/` / `host_vars/` directories are not supported (inline vars only)

## License

MIT License — free to use, modify, and distribute, including in commercial projects.
See [LICENSE](LICENSE) for the full text.
