# inv-editor — Functional Specification

## Overview

`inv-editor` is a terminal-based interactive editor for Ansible inventory files.
It supports reading and writing INI and YAML inventory formats, and provides
a keyboard-driven TUI for managing groups, hosts, and variables.

---

## Invocation

```
inv-editor <file>
```

- If `<file>` does not exist, start with an empty inventory (creates file on save).
- If `<file>` exists, detect format automatically (INI vs YAML by extension and content).

---

## Ansible Inventory Model

### Built-in Groups
| Group | Description |
|-------|-------------|
| `all` | Always present. Root group containing every host. |
| `ungrouped` | Hosts not belonging to any user-defined group. Shown only when it has members. |

### Data Model
```
Inventory
├── Groups[]
│   ├── name: string
│   ├── vars: map[string]string
│   ├── hosts: []string (references to host names)
│   └── children: []string (v1: not supported, reserved)
└── Hosts[]
    ├── name: string (IP or FQDN)
    └── vars: map[string]string
```

> **v1 scope**: Inline variables only (no `group_vars/` or `host_vars/` directory support).
> **v1 scope**: Flat groups only (no nested child groups).

---

## Supported Formats

### Read
| Format | Extension |
|--------|-----------|
| INI | `.ini`, `.cfg`, no extension |
| YAML | `.yml`, `.yaml` |

### Write / Export
| Format | Notes |
|--------|-------|
| INI | Default when original file is INI |
| YAML | Default when original file is YAML |
| Export to either format regardless of original |

---

## TUI Layout

```
┌─────────────────────────────────────────────────────────────┐
│  inv-editor: vc8.ini  [modified]           Press ? for help │
├───────────────────────┬─────────────────────────────────────┤
│  GROUPS          [G]  │  HOSTS (webservers)            [H]  │
│ ──────────────────    │ ──────────────────────────────────  │
│   all                 │   web01.example.com                 │
│ > webservers          │ > web02.example.com                 │
│   dbservers           │   web03.example.com                 │
│                       │                                     │
│   [+ New Group]       │   [+ New Host]                      │
├───────────────────────┴─────────────────────────────────────┤
│  VARIABLES  (web02.example.com)                        [V]  │
│ ──────────────────────────────────────────────────────────  │
│   ansible_user  = ubuntu                                    │
│   ansible_port  = 22                                        │
│   ansible_ssh_private_key_file = ~/.ssh/id_rsa             │
│                                                             │
│   [+ New Variable]                                          │
├─────────────────────────────────────────────────────────────┤
│  Tab next panel │ n new │ e edit │ d del │ s save │ q quit  │
└─────────────────────────────────────────────────────────────┘
```

### Panels
| Panel | Key | Description |
|-------|-----|-------------|
| Groups | `G` or `Tab` | Left panel: list of all groups |
| Hosts | `H` or `Tab` | Right panel: hosts in selected group |
| Variables | `V` or `Tab` | Bottom panel: vars for selected host or group |

The Variables panel shows:
- Host variables when a **host** is selected in the Hosts panel.
- Group variables when focus is in the **Groups** panel (no host selected).

---

## Keyboard Reference

### Global
| Key | Action |
|-----|--------|
| `Tab` / `Shift+Tab` | Cycle focus between panels (Groups → Hosts → Variables) |
| `G` | Jump focus to Groups panel |
| `H` | Jump focus to Hosts panel |
| `V` | Jump focus to Variables panel |
| `s` | Save to original file |
| `x` | Export: prompt for format and path |
| `q` | Quit (warn if unsaved changes) |
| `?` | Toggle help overlay |

### Within a Panel
| Key | Action |
|-----|--------|
| `↑` / `↓` or `j` / `k` | Navigate items |
| `n` | New item (group / host / variable) |
| `e` or `Enter` | Edit/rename selected item |
| `d` or `Delete` | Delete selected item (with confirmation) |

### Groups Panel Specifics
| Key | Action |
|-----|--------|
| `Enter` | Select group → refresh Hosts panel |
| `e` | Rename group |
| `d` | Delete group (hosts become ungrouped, confirm required) |

### Hosts Panel Specifics
| Key | Action |
|-----|--------|
| `Enter` | Select host → refresh Variables panel |
| `e` | Rename/edit host address |
| `d` | Delete host (confirm required) |
| `m` | Move host to another group (group picker prompt) |

### Variables Panel Specifics
| Key | Action |
|-----|--------|
| `n` | New variable: inline edit `key = value` |
| `e` or `Enter` | Edit selected variable (key and value separately) |
| `d` | Delete selected variable (confirm required) |

---

## Dialogs / Prompts

All dialogs are modal overlays in the center of the terminal.

### New / Edit Group
```
┌─── New Group ──────────────────┐
│ Group name: [________________] │
│                                │
│         [OK]  [Cancel]         │
└────────────────────────────────┘
```

### New / Edit Host
```
┌─── New Host ───────────────────────────────┐
│ Host address: [____________________________] │
│ (IP or FQDN)                               │
│                                            │
│              [OK]  [Cancel]                │
└────────────────────────────────────────────┘
```

### New / Edit Variable
```
┌─── New Variable ───────────────────────────┐
│ Key:   [____________________________]      │
│ Value: [____________________________]      │
│                                            │
│              [OK]  [Cancel]                │
└────────────────────────────────────────────┘
```

### Export
```
┌─── Export Inventory ─────────────────────────┐
│ Format:  ( ) INI   (•) YAML                  │
│ Path:   [vc8.yaml_____________________]      │
│                                              │
│               [Export]  [Cancel]             │
└──────────────────────────────────────────────┘
```

### Delete Confirmation
```
┌─── Confirm Delete ──────────────────────────┐
│ Delete host "web02.example.com"?            │
│ This action cannot be undone.               │
│                                             │
│           [Delete]  [Cancel]                │
└─────────────────────────────────────────────┘
```

### Quit with Unsaved Changes
```
┌─── Unsaved Changes ─────────────────────────┐
│ You have unsaved changes.                   │
│                                             │
│  [Save & Quit]  [Quit without saving]  [Cancel] │
└─────────────────────────────────────────────┘
```

---

## File Format Details

### INI Format
```ini
[webservers]
web01.example.com ansible_user=ubuntu ansible_port=22
web02.example.com

[dbservers]
db01.example.com ansible_user=postgres

[webservers:vars]
http_port=80
```

### YAML Format
```yaml
all:
  children:
    webservers:
      hosts:
        web01.example.com:
          ansible_user: ubuntu
          ansible_port: 22
        web02.example.com: {}
      vars:
        http_port: 80
    dbservers:
      hosts:
        db01.example.com:
          ansible_user: postgres
```

---

## Error Handling

| Situation | Behavior |
|-----------|----------|
| File cannot be read | Show error message and exit |
| File format unrecognized | Show error and prompt to start empty |
| Save fails (permission) | Show error dialog, do not lose in-memory state |
| Export path already exists | Prompt to overwrite or choose new path |

---

## v1 Limitations (Future Scope)

- No support for child groups (group-of-groups / `children:`)
- No support for `group_vars/` or `host_vars/` directories
- No support for inventory directories (multiple files)
- No support for dynamic inventory scripts/plugins
- No syntax validation of variable values
- No SSH connectivity test from the TUI
