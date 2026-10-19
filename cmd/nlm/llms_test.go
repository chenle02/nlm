package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// dispatchedCommands returns every command name in runCmd's switch, read from the
// source so that a new case cannot be added without this test seeing it.
func dispatchedCommands(t *testing.T) []string {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	ast.Inspect(f, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "runCmd" {
			return true
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if cc, ok := n.(*ast.CaseClause); ok {
				for _, e := range cc.List {
					if lit, ok := e.(*ast.BasicLit); ok && lit.Kind == token.STRING {
						s, _ := strconv.Unquote(lit.Value)
						names = append(names, s)
					}
				}
			}
			return true
		})
		return false
	})
	if len(names) < 20 {
		t.Fatalf("found only %d commands in runCmd; did the dispatch move?", len(names))
	}
	return names
}

// llms.txt is the agent contract: every dispatched command must be documented in its
// COMMANDS section (RELEASING.md, step 2).
func TestLLMSTxtCoversEveryCommand(t *testing.T) {
	data, err := os.ReadFile("../../llms.txt")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	start := strings.Index(text, "\nCOMMANDS")
	end := strings.Index(text, "\nWAIT FOR")
	if start < 0 || end < start {
		t.Fatal("llms.txt: no COMMANDS section followed by WAIT FOR")
	}
	section := text[start:end]
	for _, name := range dispatchedCommands(t) {
		re := regexp.MustCompile(`(?m)(^|[\s|])` + regexp.QuoteMeta(name) + `(\s|$)`)
		if !re.MatchString(section) {
			t.Errorf("llms.txt COMMANDS does not mention %q", name)
		}
	}
}

// The usage text must not advertise a command that runCmd does not dispatch (0.2.0
// listed notes, edit-note, refresh-source and check-source; none of them existed).
func TestUsageListsOnlyDispatchedCommands(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	known := map[string]bool{}
	for _, n := range dispatchedCommands(t) {
		known[n] = true
	}
	line := regexp.MustCompile(`Fprintf\(os\.Stderr, "  ([a-z][a-z-]*)(, ([a-z-]+))? `)
	for _, m := range line.FindAllStringSubmatch(string(src), -1) {
		for _, name := range []string{m[1], m[3]} {
			if name != "" && !known[name] {
				t.Errorf("usage advertises %q, which runCmd does not dispatch", name)
			}
		}
	}
}
