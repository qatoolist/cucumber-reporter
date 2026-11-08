//go:build ignore

// Checks the complete exported Go surface, including aliases not referenced by tests.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/importer"
	"go/token"
	"go/types"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
)

func main() {
	paths := []string{"github.com/qatoolist/cucumber-reporter/cucumber", "github.com/qatoolist/cucumber-reporter/report", "github.com/qatoolist/cucumber-reporter/pdf"}
	args := append([]string{"list", "-export", "-deps", "-json"}, paths...)
	raw, e := exec.Command("go", args...).Output()
	if e != nil {
		panic(e)
	}
	exports := map[string]string{}
	d := json.NewDecoder(bytes.NewReader(raw))
	for {
		var p struct{ ImportPath, Export string }
		e = d.Decode(&p)
		if e == io.EOF {
			break
		}
		if e != nil {
			panic(e)
		}
		exports[p.ImportPath] = p.Export
	}
	imp := importer.ForCompiler(token.NewFileSet(), "gc", func(path string) (io.ReadCloser, error) { return os.Open(exports[path]) })
	seen := map[types.Type]bool{}
	bad := false
	var walk func(types.Type)
	walk = func(t types.Type) {
		if t == nil || seen[t] {
			return
		}
		seen[t] = true
		switch x := t.(type) {
		case *types.Alias:
			walk(x.Rhs())
		case *types.Named:
			p := x.Obj().Pkg()
			if p != nil && strings.Contains(p.Path(), "/internal/") {
				fmt.Println("FAIL internal API type:", x)
				bad = true
				return
			}
			if p != nil && !strings.HasPrefix(p.Path(), "github.com/qatoolist/cucumber-reporter") {
				return
			}
			walk(x.Underlying())
			for i := 0; i < x.NumMethods(); i++ {
				if x.Method(i).Exported() {
					walk(x.Method(i).Type())
				}
			}
		case *types.Pointer:
			walk(x.Elem())
		case *types.Slice:
			walk(x.Elem())
		case *types.Array:
			walk(x.Elem())
		case *types.Map:
			walk(x.Key())
			walk(x.Elem())
		case *types.Chan:
			walk(x.Elem())
		case *types.Struct:
			for i := 0; i < x.NumFields(); i++ {
				walk(x.Field(i).Type())
			}
		case *types.Signature:
			walk(x.Params())
			walk(x.Results())
		case *types.Tuple:
			for i := 0; i < x.Len(); i++ {
				walk(x.At(i).Type())
			}
		case *types.Interface:
			for i := 0; i < x.NumMethods(); i++ {
				walk(x.Method(i).Type())
			}
		}
	}
	var surface []string
	for _, path := range paths {
		p, e := imp.Import(path)
		if e != nil {
			panic(e)
		}
		for _, n := range p.Scope().Names() {
			o := p.Scope().Lookup(n)
			if !o.Exported() {
				continue
			}
			walk(o.Type())
			declaration := path + " " + o.String()
			if c, ok := o.(*types.Const); ok {
				declaration += " = " + c.Val().ExactString()
			}
			surface = append(surface, declaration)
			if _, ok := o.(*types.TypeName); ok {
				if named, ok := types.Unalias(o.Type()).(*types.Named); ok {
					for i := 0; i < named.NumMethods(); i++ {
						method := named.Method(i)
						if method.Exported() {
							surface = append(surface, path+" "+n+"."+method.Name()+" "+method.String())
						}
					}
				}
			}
		}
	}
	sort.Strings(surface)
	text := strings.Join(surface, "\n") + "\n"
	file := "internal/apicheck/public-api.txt"
	if len(os.Args) > 1 && os.Args[1] == "-update" {
		if e := os.WriteFile(file, []byte(text), 0o600); e != nil {
			panic(e)
		}
	} else {
		expected, e := os.ReadFile(file)
		if e != nil || string(expected) != text {
			fmt.Println("FAIL exported API differs from reviewed snapshot")
			bad = true
		}
	}
	if bad {
		os.Exit(1)
	}
	fmt.Printf("API surface: %d exported declarations; no internal types; snapshot matches\n", len(surface))
}
