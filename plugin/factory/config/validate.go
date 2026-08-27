package config

// validate.go checks a hand-edited config before a run writes anything, and
// resolves its paths against the file it came from.
//
// Every problem is reported at once rather than the first: a config is edited in
// a batch, and surfacing one problem per run turns a single fix into several
// round trips.

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// validate reports every problem it finds at once, rather than the first.
//
// A config is edited by hand and checked in a batch, so surfacing one problem per
// run turns a single fix into several round trips.
func (c *Config) validate(source string) error {
	var problems []string
	report := func(format string, args ...any) {
		problems = append(problems, fmt.Sprintf(format, args...))
	}

	switch {
	case strings.TrimSpace(c.Version) == "":
		report("version is required (%q is the only supported value)", Version)
	case c.Version != Version:
		report("version %q is not supported by this build (expected %q)", c.Version, Version)
	}

	if len(c.Proto.Paths) == 0 && c.Proto.DescriptorSet == "" {
		report("proto needs either paths (directories of .proto files) or descriptor_set (a prebuilt FileDescriptorSet)")
	}
	if len(c.Proto.Paths) > 0 && c.Proto.DescriptorSet != "" {
		report("proto sets both paths and descriptor_set; a descriptor set is already resolved, so paths would be ignored")
	}
	if len(c.Generate) == 0 {
		report("generate is empty; name at least one target to render")
	}

	seen := map[string]bool{}
	for i, e := range c.Generate {
		if strings.TrimSpace(e.Target) == "" {
			report("generate[%d].target is empty", i)
			continue
		}
		// Two entries writing one directory would have the second silently
		// overwrite the first, and the symptom — a missing file — appears nowhere
		// near the cause.
		out := e.outDir()
		if seen[out] {
			report("generate[%d] writes to %q, which another entry already claims", i, out)
		}
		seen[out] = true
	}

	if len(problems) == 0 {
		return nil
	}
	sort.Strings(problems)
	return fmt.Errorf("%s: %d problem(s):\n  - %s", source, len(problems), strings.Join(problems, "\n  - "))
}

// resolve rewrites every path in the config to be relative to the config file's
// own directory.
func (c *Config) resolve(base string) {
	if base == "" || base == "." {
		return
	}
	for i, p := range c.Proto.Paths {
		c.Proto.Paths[i] = filepath.Join(base, p)
	}
	for i, p := range c.Proto.Imports {
		c.Proto.Imports[i] = filepath.Join(base, p)
	}
	if c.Proto.DescriptorSet != "" {
		c.Proto.DescriptorSet = filepath.Join(base, c.Proto.DescriptorSet)
	}
	c.Out = filepath.Join(base, c.Out)
	if c.Lock != "" {
		c.Lock = filepath.Join(base, c.Lock)
	}
}
