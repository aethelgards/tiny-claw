package static

import "embed"

//go:embed web/dist/*
var WebFS embed.FS
