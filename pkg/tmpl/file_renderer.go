package tmpl

import (
	"bytes"
	"fmt"
	"os"
	"strings"
)

type FileRenderer struct {
	ReadFile func(string) ([]byte, error)
	Context  *Context
	Data     interface{}
}

func NewFileRenderer(readFile func(filename string) ([]byte, error), basePath string, data interface{}) *FileRenderer {
	return &FileRenderer{
		ReadFile: readFile,
		Context: &Context{
			basePath: basePath,
			readFile: readFile,
		},
		Data: data,
	}
}

func NewFirstPassRenderer(basePath string, data interface{}) *FileRenderer {
	return &FileRenderer{
		ReadFile: os.ReadFile,
		Context: &Context{
			preRender: true,
			basePath:  basePath,
			readFile:  os.ReadFile,
		},
		Data: data,
	}
}

func (r *FileRenderer) RenderTemplateFileToBuffer(file string) (*bytes.Buffer, error) {
	content, err := r.ReadFile(file)
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
		yamlBytes, err = r.ReadFile(path)
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
