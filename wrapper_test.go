package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStoreAndList(t *testing.T) {
	dir := t.TempDir()
	ns := []Node{{"one\nline", sample}}
	if e := storeNodes(dir, ns); e != nil {
		t.Fatal(e)
	}
	s, e := listNodes(dir)
	if e != nil || strings.Count(s, "\n") != 0 || !strings.Contains(s, "one line") {
		t.Fatal(s, e)
	}
	st, e := os.Stat(filepath.Join(dir, "servers.json"))
	if e != nil || st.Mode().Perm() != 0600 {
		t.Fatal(st, e)
	}
}
func TestCLIImportStatus(t *testing.T){dir:=t.TempDir();out,e:=runCLI(dir,"unused",[]string{"import",sample});if e!=nil||!strings.Contains(out,"1"){t.Fatal(out,e)};out,e=runCLI(dir,"unused",[]string{"list"});if e!=nil||strings.Contains(out,"vless://")||!strings.Contains(out,"My server"){t.Fatal(out,e)};out,e=runCLI(dir,"unused",[]string{"status"});if e!=nil||out!="отключено"{t.Fatal(out,e)};if _,e=runCLI(dir,"unused",[]string{"disconnect"});e!=nil{t.Fatal(e)}}
func TestShellQuote(t *testing.T) {
	if shellQuote("mom's mac") != "'mom'\"'\"'s mac'" {
		t.Fatal("bad quote")
	}
}
