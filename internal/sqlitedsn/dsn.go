package sqlitedsn

import (
	"net/url"
	"path/filepath"
	"strings"
)

type Options struct {
	Mode string

	ForeignKeys bool
}

func Build(path string, opts Options) string {
	query := make(url.Values)

	query.Add("_pragma", "busy_timeout(5000)")
	if opts.ForeignKeys {
		query.Add("_pragma", "foreign_keys(1)")
	}
	query.Add("_pragma", "journal_mode(WAL)")
	if opts.Mode != "" {
		query.Set("mode", opts.Mode)
	}

	slash := filepath.ToSlash(path)
	if !strings.HasPrefix(slash, "/") {
		slash = "/" + slash
	}
	return (&url.URL{Scheme: "file", Path: slash, RawQuery: query.Encode()}).String()
}
