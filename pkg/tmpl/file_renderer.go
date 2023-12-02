package tmpl

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/helmfile/helmfile/pkg/filesystem"
)

type FileRenderer struct {
	fs      *filesystem.FileSystem
	Context *Context
	Data    any
}

func NewFileRenderer(fs *filesystem.FileSystem, basePath, rootDir string, data any) *FileRenderer {
	return &FileRenderer{
		fs: fs,
		Context: &Context{
			basePath: basePath,
			fs:       fs,
			rootDir:  rootDir,
		},
		Data: data,
	}
}

func NewFirstPassRenderer(basePath string, data any, rootDir string) *FileRenderer {
	fs := filesystem.DefaultFileSystem()
	return &FileRenderer{
		fs: fs,
		Context: &Context{
			preRender: true,
			basePath:  basePath,
			rootDir:   rootDir,
			fs:        fs,
		},
		Data: data,
	}
}

func (r *FileRenderer) RenderTemplateFileToBuffer(file string) (*bytes.Buffer, error) {
	content, err := r.fs.ReadFile(file)
	if err != nil {
		return nil, err
	}

	return r.RenderTemplateContentToBuffer(content)
}

// RenderToBytes loads the content of the file.
// If its extension is `gotmpl` it treats the content as a go template and renders it.
func (r *FileRenderer) RenderToBytes(path string) ([]byte, error) {
	var yamlBytes []byte
	splits := strings.Split(path, ".")
	if len(splits) > 0 && splits[len(splits)-1] == "gotmpl" {
		yamlBuf, err := r.RenderTemplateFileToBuffer(path)
		if err != nil {
			return nil, fmt.Errorf("failed to render [%s], because of %v", path, err)
		}
		yamlBytes = yamlBuf.Bytes()
	} else {
		var err error
		yamlBytes, err = r.fs.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to load [%s]: %v", path, err)
		}
	}
	return yamlBytes, nil
}

func (r *FileRenderer) RenderTemplateContentToBuffer(content []byte) (*bytes.Buffer, error) {
	return r.Context.RenderTemplateToBuffer(string(content), r.Data)
}

func (r *FileRenderer) RenderTemplateContentToString(content []byte) (string, error) {
	buf, err := r.Context.RenderTemplateToBuffer(string(content), r.Data)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
