package main

// flags.go holds the option types protoc's flag plumbing needs.

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
)

// stringList is a flag that accumulates rather than replacing, so an option can
// be repeated.
//
// It exists because buf splits an `opt:` entry on commas before handing it to the
// plugin: `opt: [lang=cpp,go]` arrives as two separate parameters, `lang=cpp` and
// a bare `go`, and the second is rejected as an unknown flag. A comma-separated
// value is therefore not expressible here at all, whatever the plugin does with
// it — so the option repeats instead:
//
//	opt:
//	  - lang=cpp
//	  - lang=go
type stringList []string

// String renders the accumulated values, as flag.Value requires.
func (l *stringList) String() string { return strings.Join(*l, ",") }

// Set appends rather than replacing, which is what makes the option repeatable.
func (l *stringList) Set(v string) error {
	if v = strings.TrimSpace(v); v != "" {
		*l = append(*l, v)
	}
	return nil
}

// protocVersion renders the compiler version from the request, for the banner.
func protocVersion(p *protogen.Plugin) string {
	v := p.Request.GetCompilerVersion()
	if v == nil {
		return ""
	}
	return fmt.Sprintf("v%d.%d.%d", v.GetMajor(), v.GetMinor(), v.GetPatch())
}
