package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var embeddedDist embed.FS

func DistFS() (fs.FS, error) {
	return fs.Sub(embeddedDist, "dist")
}
