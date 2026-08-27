package config

// paths.go answers where a target's output goes.

import "path/filepath"

// OutDir returns the absolute-ish directory an entry's schema is written to.
func (c *Config) OutDir(e Entry) string { return filepath.Join(c.Out, e.outDir()) }

// LangDir returns the directory an entry's compiled language output goes to.
func (c *Config) LangDir(e Entry, lang string) string {
	if e.LangOut != "" {
		return filepath.Join(c.Out, e.LangOut, lang)
	}
	return filepath.Join(c.Out, e.Target+"-"+lang)
}

// ModuleFor returns the Go module root for an entry, preferring its own override.
func (c *Config) ModuleFor(e Entry) string {
	if e.GoModule != "" {
		return e.GoModule
	}
	return c.GoModule
}

// outDir returns the entry's directory name, defaulting to the target's — which
// is what keeps two entries from colliding.
func (e Entry) outDir() string {
	if e.Out != "" {
		return e.Out
	}
	return e.Target
}
