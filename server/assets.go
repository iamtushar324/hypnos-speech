package speech

import (
	"embed"
	"io/fs"
)

//go:embed web
var assets embed.FS

func WebAssets() fs.FS {
	root, err := fs.Sub(assets, "web")
	if err != nil {
		panic(err)
	}
	return root
}
