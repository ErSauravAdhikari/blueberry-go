package rasberry

import (
	"embed"
	"github.com/labstack/echo/v4"
	"html/template"
	"io"
)

//go:embed templates/*
var templatesFS embed.FS

type Template struct {
	templates *template.Template
}

func (t *Template) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

func loadTemplates() (*template.Template, error) {
	t, err := template.ParseFS(templatesFS, "templates/*.goml")
	if err != nil {
		return nil, err
	}
	return t, nil
}
