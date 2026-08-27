package langs

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// capnpschema.go locates the annotation schemas some capnp generators require.
//
// A schema emitting `using Go = import "/go.capnp";` does not compile — in any
// language — unless go.capnp is on the import path. That file ships inside the
// capnpc-go Go module rather than anywhere on the system include path, so it has
// to be found rather than assumed.
//
// Getting this wrong produces a confusing failure: `capnp compile -o c++` fails
// with "file not found: /go.capnp" on a schema that has nothing to do with Go.
// So the lookup happens here, once, and its result is added to -I for every
// capnp invocation in the run.

// annotationImportPath returns a directory to add to capnp's import path for the
// given language's annotation schema, and whether one was found.
//
// A miss is not an error here. The caller compiles anyway and lets capnp produce
// its own diagnostic, which names the missing import and is more useful than
// anything this function could say about a toolchain it did not install.
func annotationImportPath(lang string) (string, bool) {
	switch lang {
	case "go":
		return goCapnpDir()
	case "java":
		return javaCapnpDir()
	}
	return "", false
}

// goCapnpDir finds the directory holding go.capnp.
//
// capnpc-go ships it at std/go.capnp inside its module. The module cache path
// carries a version, so it is globbed rather than constructed — pinning a version
// here would break on the next release of a module this repository does not
// depend on.
func goCapnpDir() (string, bool) {
	cache := goEnv("GOMODCACHE")
	if cache == "" {
		return "", false
	}

	// v3 first: it is the current major and the one `go install …/v3/capnpc-go`
	// puts on disk. The v2 path is checked as a fallback because the v3 module
	// still pulls it in, and a machine may have only that.
	for _, pattern := range []string{
		filepath.Join(cache, "capnproto.org", "go", "capnp", "v3@*", "std", "go.capnp"),
		filepath.Join(cache, "capnproto.org", "go", "capnp@*", "std", "go.capnp"),
	} {
		matches, err := filepath.Glob(pattern)
		if err != nil || len(matches) == 0 {
			continue
		}
		// Several versions may be cached; the last of a sorted glob is the
		// highest, and go.capnp is stable across them anyway.
		return filepath.Dir(highest(matches)), true
	}
	return "", false
}

// javaCapnpDir finds the directory holding capnp/java.capnp.
//
// capnproto-java installs it alongside the compiler plugin, conventionally under
// a prefix's include/. There is no module cache to consult, so the usual prefixes
// are checked directly.
func javaCapnpDir() (string, bool) {
	var candidates []string
	if prefix := os.Getenv("CAPNP_JAVA_PREFIX"); prefix != "" {
		candidates = append(candidates, filepath.Join(prefix, "include"))
	}
	candidates = append(candidates,
		"/usr/local/include",
		"/usr/include",
		"/opt/homebrew/include",
	)
	for _, dir := range candidates {
		if _, err := os.Stat(filepath.Join(dir, "capnp", "java.capnp")); err == nil {
			return dir, true
		}
	}
	return "", false
}

// goEnv reads one `go env` value, returning "" when the Go toolchain is absent.
func goEnv(name string) string {
	out, err := exec.Command("go", "env", name).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// highest returns the lexically greatest path, which for a version-suffixed
// module cache glob is the newest version.
func highest(paths []string) string {
	best := paths[0]
	for _, p := range paths[1:] {
		if p > best {
			best = p
		}
	}
	return best
}
