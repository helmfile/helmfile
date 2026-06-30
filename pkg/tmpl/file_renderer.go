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

// FileRendererOption configures a FileRenderer's Context.
type FileRendererOption func(*Context)

// WithPreRender configures the renderer to run in pre-render mode,
// disabling side-effect template functions (exec, readFile, etc.).
func WithPreRender() FileRendererOption {
	return func(c *Context) { c.preRender = true }
}

// WithLenientRequiredEnv configures the renderer to treat requiredEnv leniently:
// unset/empty environment variables produce empty strings instead of failing.
func WithLenientRequiredEnv() FileRendererOption {
	return func(c *Context) { c.lenientRequiredEnv = true }
}

func NewFileRenderer(fs *filesystem.FileSystem, basePath string, data any) *FileRenderer {
	return newFileRenderer(fs, basePath, data)
}

func NewFirstPassRenderer(basePath string, data any) *FileRenderer {
	return newFileRenderer(filesystem.DefaultFileSystem(), basePath, data, WithPreRender())
}

// NewLenientFileRenderer creates a FileRenderer that treats requiredEnv leniently.
// This is used when selectors are active so that releases excluded by selectors
// do not block rendering of the whole document.
// See https://github.com/helmfile/helmfile/issues/1172
func NewLenientFileRenderer(fs *filesystem.FileSystem, basePath string, data any) *FileRenderer {
	return newFileRenderer(fs, basePath, data, WithLenientRequiredEnv())
}

func newFileRenderer(fs *filesystem.FileSystem, basePath string, data any, opts ...FileRendererOption) *FileRenderer {
	ctx := &Context{basePath: basePath, fs: fs}
	for _, opt := range opts {
		opt(ctx)
	}
	return &FileRenderer{fs: fs, Context: ctx, Data: data}
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
