package rasberry

import (
	"embed"
	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/log"
	"html/template"
	"io"
	"time"
)

//go:embed templates/*.goml
var content embed.FS

type Template struct {
	templates *template.Template
}

func (t *Template) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	err := t.templates.ExecuteTemplate(w, name, data)
	if err != nil {
		log.Printf("Error rendering template %s: %v", name, err)
	}
	return err
}

func loadTemplates() (*template.Template, error) {
	t := template.New("").Funcs(template.FuncMap{
		"date": func(t time.Time) string {
			return t.Format("02-Jan-2006")
		},
	})

	t, err := template.ParseFS(content, "templates/*.goml")
	if err != nil {
		return nil, err
	}
	return t, nil
}
