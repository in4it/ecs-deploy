package ngserve

// modified from https://github.com/gin-contrib/static/blob/master/static.go
// to redirect "404 - not found" to Angular's index.html

import (
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
)

type ServeFileSystem interface {
	http.FileSystem
	Exists(prefix string, path string) bool
}

type localFileSystem struct {
	http.FileSystem
	root    string
	indexes bool
}

func LocalFile(root string, indexes bool) *localFileSystem {
	return &localFileSystem{
		FileSystem: gin.Dir(root, indexes),
		root:       root,
		indexes:    indexes,
	}
}

func (l *localFileSystem) Exists(prefix string, filepath string) bool {
	if p := strings.TrimPrefix(filepath, prefix); len(p) < len(filepath) {
		name := path.Join(l.root, p)
		stats, err := os.Stat(name)
		if err != nil {
			return false
		}
		if !l.indexes && stats.IsDir() {
			return false
		}
		return true
	}
	return false
}

// Static returns a middleware handler that serves static files in the given directory.
func ServeWithDefault(urlPrefix string, fs ServeFileSystem, defaultFile string) gin.HandlerFunc {
	fileserver := http.FileServer(fs)
	if urlPrefix != "" {
		fileserver = http.StripPrefix(urlPrefix, fileserver)
	}
	return func(c *gin.Context) {
		if fs.Exists(urlPrefix, c.Request.URL.Path) {
			fileserver.ServeHTTP(c.Writer, c.Request)
			c.Abort()
		} else {
			if strings.HasPrefix(c.Request.URL.Path, urlPrefix) {
				// doesn't exist, but matches files in directory
				// serve default
				http.ServeFile(c.Writer, c.Request, defaultFile)
			}
		}
	}
}
