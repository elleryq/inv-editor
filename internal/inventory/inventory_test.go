package inventory

import (
	"strings"
	"testing"
)

func TestParseINI(t *testing.T) {
	data := []byte(`
[webservers]
web01.example.com ansible_user=ubuntu ansible_port=22
web02.example.com

[dbservers]
db01.example.com ansible_user=postgres

[webservers:vars]
http_port=80
`)
	inv, err := ParseINI(data)
	if err != nil {
		t.Fatal(err)
	}

	groups := inv.Groups()
	// should have: all, webservers, dbservers
	if len(groups) != 3 {
		t.Errorf("expected 3 groups, got %d", len(groups))
	}
	if groups[0].Name != "all" {
		t.Errorf("first group should be 'all', got %q", groups[0].Name)
	}

	web := inv.Group("webservers")
	if web == nil {
		t.Fatal("webservers group not found")
	}
	if web.Vars["http_port"] != "80" {
		t.Errorf("expected http_port=80, got %q", web.Vars["http_port"])
	}

	h := inv.Host("web01.example.com")
	if h == nil {
		t.Fatal("web01 host not found")
	}
	if h.Vars["ansible_user"] != "ubuntu" {
		t.Errorf("expected ansible_user=ubuntu, got %q", h.Vars["ansible_user"])
	}
	if h.Vars["ansible_port"] != "22" {
		t.Errorf("expected ansible_port=22, got %q", h.Vars["ansible_port"])
	}
}

func TestWriteINI(t *testing.T) {
	inv := New()
	inv.AddGroup("webservers")
	inv.AddHost("web01.example.com", "webservers")
	inv.SetHostVar("web01.example.com", "ansible_user", "ubuntu")
	inv.SetGroupVar("webservers", "http_port", "80")

	var sb strings.Builder
	// write to temp file then read back
	err := WriteINIFile(inv, "/tmp/inv_test_out.ini")
	if err != nil {
		t.Fatal(err)
	}
	_ = sb

	inv2, err := ParseINIFile("/tmp/inv_test_out.ini")
	if err != nil {
		t.Fatal(err)
	}
	h := inv2.Host("web01.example.com")
	if h == nil || h.Vars["ansible_user"] != "ubuntu" {
		t.Error("host vars not preserved through INI round-trip")
	}
	g := inv2.Group("webservers")
	if g == nil || g.Vars["http_port"] != "80" {
		t.Error("group vars not preserved through INI round-trip")
	}
}

func TestParseYAML(t *testing.T) {
	data := []byte(`
all:
  children:
    webservers:
      hosts:
        web01.example.com:
          ansible_user: ubuntu
          ansible_port: "22"
      vars:
        http_port: "80"
    dbservers:
      hosts:
        db01.example.com:
          ansible_user: postgres
`)
	inv, err := ParseYAML(data)
	if err != nil {
		t.Fatal(err)
	}

	web := inv.Group("webservers")
	if web == nil {
		t.Fatal("webservers not found")
	}
	if web.Vars["http_port"] != "80" {
		t.Errorf("expected http_port=80, got %q", web.Vars["http_port"])
	}

	h := inv.Host("web01.example.com")
	if h == nil {
		t.Fatal("web01 not found")
	}
	if h.Vars["ansible_user"] != "ubuntu" {
		t.Errorf("expected ansible_user=ubuntu, got %q", h.Vars["ansible_user"])
	}
}

func TestYAMLRoundTrip(t *testing.T) {
	inv := New()
	inv.AddGroup("webservers")
	inv.AddHost("web01.example.com", "webservers")
	inv.SetHostVar("web01.example.com", "ansible_user", "ubuntu")
	inv.SetGroupVar("webservers", "http_port", "80")

	err := WriteYAMLFile(inv, "/tmp/inv_test_out.yaml")
	if err != nil {
		t.Fatal(err)
	}

	inv2, err := ParseYAMLFile("/tmp/inv_test_out.yaml")
	if err != nil {
		t.Fatal(err)
	}
	h := inv2.Host("web01.example.com")
	if h == nil || h.Vars["ansible_user"] != "ubuntu" {
		t.Error("host vars not preserved through YAML round-trip")
	}
	g := inv2.Group("webservers")
	if g == nil || g.Vars["http_port"] != "80" {
		t.Error("group vars not preserved through YAML round-trip")
	}
}

func TestModelOperations(t *testing.T) {
	inv := New()
	inv.AddGroup("web")
	inv.AddGroup("db")
	inv.AddHost("h1", "web")
	inv.AddHost("h2", "web")
	inv.AddHost("h3", "db")

	// move h2 from web to db
	if !inv.MoveHost("h2", "web", "db") {
		t.Error("MoveHost failed")
	}
	webHosts := inv.HostsInGroup("web")
	if len(webHosts) != 1 || webHosts[0].Name != "h1" {
		t.Errorf("unexpected web hosts after move: %v", webHosts)
	}

	// delete group
	if !inv.DeleteGroup("db") {
		t.Error("DeleteGroup failed")
	}
	if inv.Group("db") != nil {
		t.Error("group db should be gone")
	}
	// h2 and h3 hosts still exist
	if inv.Host("h2") == nil {
		t.Error("host h2 should still exist after group deletion")
	}
}

func TestCopyGroupDeep(t *testing.T) {
	inv := New()
	inv.AddGroup("frontend")
	inv.AddGroupUnder("frontend", "frontend-web")
	inv.AddHost("h1", "frontend")
	inv.AddHost("h2", "frontend-web")
	inv.SetGroupVar("frontend", "env", "prod")

	inv.AddGroup("staging")

	newName := inv.CopyGroupDeep("frontend", "staging", "frontend-copy")
	if newName != "frontend-copy" {
		t.Fatalf("expected new root name %q, got %q", "frontend-copy", newName)
	}

	root := inv.Group("frontend-copy")
	if root == nil {
		t.Fatal("copied root group not found")
	}
	if root.Parent != "staging" {
		t.Errorf("expected copied root parented under staging, got %q", root.Parent)
	}
	if root.Vars["env"] != "prod" {
		t.Errorf("expected copied vars to carry over, got %v", root.Vars)
	}
	if len(root.Children) != 1 {
		t.Fatalf("expected 1 child group, got %d", len(root.Children))
	}
	childName := root.Children[0]
	if childName == "frontend-web" {
		t.Error("expected child group name to be auto-suffixed to avoid collision with original")
	}
	child := inv.Group(childName)
	if child == nil || child.Parent != "frontend-copy" {
		t.Fatalf("copied child group missing or misparented: %v", child)
	}

	rootHosts := inv.HostsInGroup("frontend-copy")
	if len(rootHosts) != 1 || rootHosts[0].Name != "h1" {
		t.Errorf("unexpected hosts on copied root: %v", rootHosts)
	}
	childHosts := inv.HostsInGroup(childName)
	if len(childHosts) != 1 || childHosts[0].Name != "h2" {
		t.Errorf("unexpected hosts on copied child: %v", childHosts)
	}

	// original subtree must be untouched
	orig := inv.Group("frontend")
	if len(orig.Children) != 1 || orig.Children[0] != "frontend-web" {
		t.Errorf("original group's children mutated: %v", orig.Children)
	}
}

// TestCopyGroupDeepUnderSelf guards the snapshot-before-mutate logic in
// clone(): copying a group as a child of itself must not pick up the
// newly-created clone as one of its own children to recurse into.
func TestCopyGroupDeepUnderSelf(t *testing.T) {
	inv := New()
	inv.AddGroup("frontend")
	inv.AddGroupUnder("frontend", "frontend-web")

	newName := inv.CopyGroupDeep("frontend", "frontend", "frontend-copy")
	if newName != "frontend-copy" {
		t.Fatalf("expected new root name %q, got %q", "frontend-copy", newName)
	}
	root := inv.Group("frontend-copy")
	if root == nil || root.Parent != "frontend" {
		t.Fatalf("copied root missing or misparented: %v", root)
	}
	if len(root.Children) != 1 {
		t.Fatalf("expected exactly 1 cloned child, got %d: %v", len(root.Children), root.Children)
	}
}

func TestUniqueGroupName(t *testing.T) {
	inv := New()
	inv.AddGroup("web")
	inv.AddGroup("web-copy")

	if got := inv.UniqueGroupName("db"); got != "db" {
		t.Errorf("expected unused name unchanged, got %q", got)
	}
	if got := inv.UniqueGroupName("web"); got != "web-copy2" {
		t.Errorf("expected next free suffix, got %q", got)
	}
}

func TestMergeFrom(t *testing.T) {
	inv := New()
	inv.AddGroup("web")
	inv.AddHost("h1", "web")
	inv.SetGroupVar("web", "env", "prod") // existing value must win
	inv.SetHostVar("h1", "ansible_user", "deploy")

	other := New()
	other.AddGroup("web")
	other.AddGroupUnder("web", "web-east")          // new child group under existing "web"
	other.AddHost("h1", "web")                      // same host, already in "web"
	other.AddHost("h2", "web-east")                 // new host
	other.SetGroupVar("web", "env", "staging")      // should NOT override inv's "prod"
	other.SetGroupVar("web", "http_port", "8080")   // new var, should be added
	other.SetHostVar("h1", "ansible_user", "other") // should NOT override inv's "deploy"
	other.SetHostVar("h1", "ansible_port", "2222")  // new var, should be added

	newGroups, newHosts := inv.MergeFrom(other)
	if newGroups != 1 {
		t.Errorf("expected 1 new group (web-east), got %d", newGroups)
	}
	if newHosts != 1 {
		t.Errorf("expected 1 new host (h2), got %d", newHosts)
	}

	web := inv.Group("web")
	if web.Vars["env"] != "prod" {
		t.Errorf("existing group var should win on conflict, got %q", web.Vars["env"])
	}
	if web.Vars["http_port"] != "8080" {
		t.Errorf("new group var should be merged in, got %v", web.Vars)
	}

	webEast := inv.Group("web-east")
	if webEast == nil || webEast.Parent != "web" {
		t.Fatalf("expected new child group web-east under web, got %v", webEast)
	}
	if len(inv.HostsInGroup("web-east")) != 1 || inv.HostsInGroup("web-east")[0].Name != "h2" {
		t.Errorf("expected h2 in web-east: %v", inv.HostsInGroup("web-east"))
	}

	h1 := inv.Host("h1")
	if h1.Vars["ansible_user"] != "deploy" {
		t.Errorf("existing host var should win on conflict, got %q", h1.Vars["ansible_user"])
	}
	if h1.Vars["ansible_port"] != "2222" {
		t.Errorf("new host var should be merged in, got %v", h1.Vars)
	}

	webHosts := inv.HostsInGroup("web")
	if len(webHosts) != 1 || webHosts[0].Name != "h1" {
		t.Errorf("h1 should not be duplicated in web's host list: %v", webHosts)
	}
}

func TestSubgroupINIRoundTrip(t *testing.T) {
	data := []byte(`
[production]
prod01.example.com

[staging]
stg01.example.com

[webservers:children]
production
staging

[webservers:vars]
http_port=80
`)
	inv, err := ParseINI(data)
	if err != nil {
		t.Fatal(err)
	}

	web := inv.Group("webservers")
	if web == nil {
		t.Fatal("webservers not found")
	}
	if len(web.Children) != 2 {
		t.Errorf("expected 2 children, got %d: %v", len(web.Children), web.Children)
	}

	prod := inv.Group("production")
	if prod == nil {
		t.Fatal("production not found")
	}
	if prod.Parent != "webservers" {
		t.Errorf("expected parent=webservers, got %q", prod.Parent)
	}
	if web.Vars["http_port"] != "80" {
		t.Errorf("expected http_port=80, got %q", web.Vars["http_port"])
	}

	// round-trip through INI
	if err := WriteINIFile(inv, "/tmp/subgroup_test.ini"); err != nil {
		t.Fatal(err)
	}
	inv2, err := ParseINIFile("/tmp/subgroup_test.ini")
	if err != nil {
		t.Fatal(err)
	}
	web2 := inv2.Group("webservers")
	if web2 == nil || len(web2.Children) != 2 {
		t.Errorf("children not preserved: %v", web2)
	}
	if inv2.Group("production").Parent != "webservers" {
		t.Error("parent not preserved after INI round-trip")
	}
}

func TestSubgroupYAMLRoundTrip(t *testing.T) {
	inv := New()
	inv.AddGroup("webservers")
	inv.AddGroupUnder("webservers", "frontend")
	inv.AddGroupUnder("webservers", "backend")
	inv.AddHost("fe01.example.com", "frontend")
	inv.AddHost("be01.example.com", "backend")
	inv.SetGroupVar("webservers", "http_port", "80")
	inv.SetHostVar("fe01.example.com", "ansible_user", "ubuntu")

	if err := WriteYAMLFile(inv, "/tmp/subgroup_test.yaml"); err != nil {
		t.Fatal(err)
	}

	inv2, err := ParseYAMLFile("/tmp/subgroup_test.yaml")
	if err != nil {
		t.Fatal(err)
	}

	web := inv2.Group("webservers")
	if web == nil {
		t.Fatal("webservers not found after YAML round-trip")
	}
	if len(web.Children) != 2 {
		t.Errorf("expected 2 children, got %d: %v", len(web.Children), web.Children)
	}

	fe := inv2.Group("frontend")
	if fe == nil || fe.Parent != "webservers" {
		t.Error("frontend parent not preserved")
	}
	if inv2.Host("fe01.example.com") == nil {
		t.Error("fe01 not found after YAML round-trip")
	}
	if inv2.Host("fe01.example.com").Vars["ansible_user"] != "ubuntu" {
		t.Error("host var not preserved")
	}
}

func TestDeleteGroupReparent(t *testing.T) {
	inv := New()
	inv.AddGroup("webservers")
	inv.AddGroupUnder("webservers", "frontend")
	inv.AddGroupUnder("webservers", "backend")

	// delete middle group — children should become top-level
	if !inv.DeleteGroup("webservers") {
		t.Fatal("DeleteGroup failed")
	}
	if inv.Group("webservers") != nil {
		t.Error("webservers should be gone")
	}
	fe := inv.Group("frontend")
	if fe == nil {
		t.Fatal("frontend should still exist")
	}
	if fe.Parent != "" {
		t.Errorf("frontend should be top-level after parent deleted, got parent=%q", fe.Parent)
	}
}
