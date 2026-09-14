// Package unit holds the architecture guard tests.
//
// These implement the layer and module-boundary rules from docs/build/05-architecture.md
// sections 2.2 and 2.5. They are a test rather than a shell lint so the rules are
// enforced by parsing real Go imports instead of matching text.
package unit

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const backendModule = "github.com/rootlogic-lab/delivery/backend"

// backendRoot resolves the backend module directory from the tests module.
func backendRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../../")
	if err != nil {
		t.Fatalf("resolve backend root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("backend go.mod not found at %s: %v", root, err)
	}
	return root
}

// goFile is one parsed source file with the import paths it declares.
type goFile struct {
	path    string
	pkgDir  string
	imports []string
}

// collectGoFiles parses every non-test Go file under the backend module.
func collectGoFiles(t *testing.T, root string) []goFile {
	t.Helper()
	var out []goFile
	fset := token.NewFileSet()

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "tests" || d.Name() == "vendor" || strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		f, perr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if perr != nil {
			return perr
		}
		imports := make([]string, 0, len(f.Imports))
		for _, spec := range f.Imports {
			p, uerr := strconv.Unquote(spec.Path.Value)
			if uerr != nil {
				return uerr
			}
			imports = append(imports, p)
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return rerr
		}
		out = append(out, goFile{
			path:    filepath.ToSlash(rel),
			pkgDir:  filepath.ToSlash(filepath.Dir(rel)),
			imports: imports,
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk backend: %v", err)
	}
	return out
}

// layerOf returns the clean-architecture layer a package directory belongs to,
// and the module that owns it. Empty layer means the path is not module code.
func layerOf(pkgDir string) (module, layer string) {
	const prefix = "internal/modules/"
	if !strings.HasPrefix(pkgDir, prefix) {
		return "", ""
	}
	rest := strings.TrimPrefix(pkgDir, prefix)
	parts := strings.Split(rest, "/")
	if len(parts) < 2 {
		return parts[0], ""
	}
	return parts[0], parts[1]
}

// internalImport reports whether an import path points inside the backend module,
// returning the package directory relative to the module root.
func internalImport(path string) (string, bool) {
	if !strings.HasPrefix(path, backendModule+"/") {
		return "", false
	}
	return strings.TrimPrefix(path, backendModule+"/"), true
}

// TestDomainImportsNothingInternal enforces 2.2: domain/ imports nothing outside itself.
func TestDomainImportsNothingInternal(t *testing.T) {
	root := backendRoot(t)
	for _, f := range collectGoFiles(t, root) {
		mod, layer := layerOf(f.pkgDir)
		if layer != "domain" {
			continue
		}
		for _, imp := range f.imports {
			dep, ok := internalImport(imp)
			if !ok {
				continue
			}
			depMod, depLayer := layerOf(dep)
			if depMod == mod && depLayer == "domain" {
				continue
			}
			t.Errorf("%s: domain must import nothing outside itself, found %q", f.path, imp)
		}
	}
}

// TestApplicationImportsDomainOnly enforces 2.2: application/ may import domain only.
func TestApplicationImportsDomainOnly(t *testing.T) {
	root := backendRoot(t)
	for _, f := range collectGoFiles(t, root) {
		mod, layer := layerOf(f.pkgDir)
		if layer != "application" {
			continue
		}
		for _, imp := range f.imports {
			dep, ok := internalImport(imp)
			if !ok {
				continue
			}
			depMod, depLayer := layerOf(dep)
			if strings.HasPrefix(dep, "internal/shared") {
				continue
			}
			// Within its own module, application may reach domain, its own
			// contract (which it implements), and external/.
			//
			// external/ is the point: 2.5 requires that extracting a module
			// into a real service changes only that one file, which is only
			// true if the application layer calls other modules *through* it.
			// Allowing it here does not weaken the cross-module wall —
			// TestCrossModuleImportsGoThroughExternal still requires that
			// external/ is the only thing reaching another module, and that it
			// reaches only that module's contract.
			if depMod == mod && (depLayer == "domain" || depLayer == "application" ||
				depLayer == "contract" || depLayer == "external") {
				continue
			}
			t.Errorf("%s: application may import only its own domain, contract, external and internal/shared, found %q", f.path, imp)
		}
	}
}

// TestTransportDoesNotImportInfrastructure enforces 2.2: transport/ imports application only.
func TestTransportDoesNotImportInfrastructure(t *testing.T) {
	root := backendRoot(t)
	for _, f := range collectGoFiles(t, root) {
		_, layer := layerOf(f.pkgDir)
		if layer != "transport" {
			continue
		}
		for _, imp := range f.imports {
			dep, ok := internalImport(imp)
			if !ok {
				continue
			}
			if _, depLayer := layerOf(dep); depLayer == "infrastructure" {
				t.Errorf("%s: transport must not import infrastructure, found %q", f.path, imp)
			}
		}
	}
}

// TestCrossModuleImportsGoThroughExternal enforces 2.5: a module never imports
// another module's domain, application, infrastructure or transport. The only
// permitted cross-module reference is from a module's own external/ package.
func TestCrossModuleImportsGoThroughExternal(t *testing.T) {
	root := backendRoot(t)
	for _, f := range collectGoFiles(t, root) {
		mod, layer := layerOf(f.pkgDir)
		if mod == "" {
			continue
		}
		for _, imp := range f.imports {
			dep, ok := internalImport(imp)
			if !ok {
				continue
			}
			depMod, depLayer := layerOf(dep)
			if depMod == "" || depMod == mod {
				continue
			}
			if layer == "external" && depLayer == "contract" {
				continue
			}
			t.Errorf("%s: cross-module import must go through external/ to a contract package, found %q", f.path, imp)
		}
	}
}
