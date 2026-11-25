inv-editor 1 "@DATE@" "inv-editor @VERSION@" "User Commands"
=============================================================

# NAME

inv-editor - interactive editor for Ansible inventory files

# SYNOPSIS

**inv-editor** *inventory-file*

**inv-editor serve** [**--host** *address*] [**--port** *port*] [**--readonly**] *inventory-file*

**inv-editor help** | **--help** | **-h**

**inv-editor version** | **--version**

# DESCRIPTION

**inv-editor** edits Ansible inventory files in INI or YAML format. It has two
interfaces:

- a keyboard-driven terminal user interface (TUI), started when the only
  argument is an inventory file;
- a web interface, started with the **serve** command.

Both manage groups (including nested child groups), hosts and variables of
groups and hosts, and write the result back in the file's original format.

If *inventory-file* does not exist, **inv-editor** starts with an empty
inventory and creates the file when it is saved. The format is detected from
the file extension and content; see **FILES**.

# COMMANDS

*inventory-file*
:   Open the inventory in the terminal user interface. See **TUI KEYS**.

**serve** [*options*] *inventory-file*
:   Start the web interface for the inventory. Open the printed address in a
    browser. Changes are written back to *inventory-file* when saved from the
    page. See **OPTIONS** and **SECURITY**.

**help**, **--help**, **-h**
:   Print a usage summary and exit.

**version**, **--version**
:   Print the version as **inv-editor** *version* and exit.

# OPTIONS

These options apply to **serve** and may be given before or after
*inventory-file*. Versions before 1.5.0 ignore options given after it.

**--host** *address*
:   Address to listen on. Default: **127.0.0.1**.

**--port** *port*
:   TCP port to listen on. Default: **8080**.

**--readonly**
:   Start in read-only mode. Edit controls are hidden and every request that
    would change the inventory is rejected with HTTP 403.

# TUI KEYS

The screen has three panels: **Groups** (a tree of groups), **Hosts** (the
hosts of the selected group) and **Variables** (of the selected group or
host). A status bar at the bottom shows messages and hints.

## Panels and navigation

**Tab**, **Shift+Tab**
:   Move focus to the next or previous panel.

**G**, **H**, **V**
:   Jump to the Groups, Hosts or Variables panel.

**Up**, **Down**, **k**, **j**
:   Move the cursor in the focused panel.

**Right**, **l**
:   In the Groups panel, expand the selected group.

**Left**, **h**
:   In the Groups panel, collapse the selected group, or jump to its parent
    if it is already collapsed.

**Shift+Left**, **Shift+Right**
:   Scroll the focused panel horizontally, for long names and values.

## Editing

**n**
:   Create a new item in the focused panel: a group, a host (hostname and an
    optional IP address, stored as **ansible_host**) or a variable.

**e**, **Enter**
:   Edit the selected item.

**d**, **Delete**
:   Delete the selected item, after confirmation. In the Hosts panel this
    removes the host from the selected group.

**v**
:   Show the variables of the selected group or host in the Variables panel.

## Copy, move and paste

**Space**
:   In the Hosts panel, toggle selection of the host under the cursor, to act
    on several hosts at once.

**c**
:   Mark the selected host(s), or the selected group, to be copied.

**m**
:   Mark the selected host(s), or the selected group, to be moved.

**p**
:   In the Groups panel, paste the marked host(s) or group into the selected
    group. Copying a host adds it to the target group and keeps it in the
    source group; moving a host removes it from the source group. Copying a
    group deep-copies it (child groups, hosts and variables) under the target
    and asks for a new name; moving a group makes it a child of the target.

**Esc**
:   Clear an active search filter, or else clear the marked items.

## Search

**/**
:   Start a search; type the query and press **Enter** to keep the filter and
    return to the Groups panel, or **Esc** to cancel it. The query is a
    regular expression (Go RE2 syntax) matched against group and host names.
    A query of the form *key*=*pattern* instead matches hosts whose variable
    *key* has a value matching *pattern*. While the expression is invalid,
    the status bar shows **invalid regex** and the lists are not filtered.

## Saving, exporting and importing

**s**
:   Save to the original file, in its original format.

**x**
:   Export to another path and format (INI or YAML).

**i**
:   Import another inventory file and merge it into the current one. Groups
    and hosts are matched by name; on a variable conflict the current value
    is kept; group membership of hosts is combined.

## Other

**?**
:   Show the help overlay; any key closes it.

**q**, **Ctrl+C**
:   Quit. If there are unsaved changes, a dialog offers **s** (save and
    quit), **d** (discard and quit) and **c**, **q** or **Esc** (cancel).

# FILES

*inventory-file*
:   An Ansible inventory. Files ending in **.ini** or **.cfg** are read and
    written as INI; files ending in **.yml** or **.yaml** as YAML. The web
    interface's **/download** endpoint always returns YAML, whatever the
    source format.

# SECURITY

The web interface has no authentication. Anyone who can reach the listening
address can read the inventory, including variables such as passwords stored
in it, and can modify it unless **--readonly** is given.

By default **serve** listens only on **127.0.0.1**. Before using
**--host 0.0.0.0** or another reachable address, consider combining it with
**--readonly**, restricting access with a firewall, or putting an
authenticating reverse proxy in front of it.

# EXAMPLES

Edit an INI inventory in the terminal:

    inv-editor production.ini

Create a new YAML inventory:

    inv-editor new-inventory.yaml

Browse an inventory in a browser on port 9090:

    inv-editor serve --port 9090 vc8.yaml

Share a read-only view on the local network:

    inv-editor serve --host 0.0.0.0 --readonly vc8.yaml

# SEE ALSO

**ansible**(1), **ansible-inventory**(1)

Project home page: <https://github.com/elleryq/inv-editor>

# BUGS

Report bugs at <https://github.com/elleryq/inv-editor/issues>.
