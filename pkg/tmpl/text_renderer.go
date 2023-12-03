package tmpl

import (
	"github.com/helmfile/helmfile/pkg/filesystem"
)

type templateTextRenderer struct {
	ReadText func(string) ([]byte, error)
	Context  *Context
	Data     any
}

type TextRenderer interface {
	RenderTemplateText(text string) (string, error)
}

func NewTextRenderer(fs *filesystem.FileSystem, basePath string, data any) *templateTextRenderer {
	return &templateTextRenderer{
		ReadText: fs.ReadFile,
		Context: &Context{
			basePath: basePath,
			fs:       fs,
		},
		Data: data,
	}
}

func (r *templateTextRenderer) RenderTemplateText(text string) (string, error) {
	buf, err := r.Context.RenderTemplateToBuffer(text, r.Data)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
