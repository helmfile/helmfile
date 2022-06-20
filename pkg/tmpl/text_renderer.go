package tmpl

type templateTextRenderer struct {
	ReadText func(string) ([]byte, error)
	Context  *Context
	Data     interface{}
}

type TextRenderer interface {
	RenderTemplateText(text string) (string, error)
}

// nolint: golint
func NewTextRenderer(readFile func(filename string) ([]byte, error), basePath string, data interface{}) *templateTextRenderer {
	return &templateTextRenderer{
		ReadText: readFile,
		Context: &Context{
			basePath: basePath,
			readFile: readFile,
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
