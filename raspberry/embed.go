package rasberry

import (
	"embed"
	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/log"
	"html/template"
	"io"
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

func add(a, b int) int {
	return a + b
}

func sub(a, b int) int {
	return a - b
}

func loadTemplates() (*template.Template, error) {
	t := template.Must(template.New("").Funcs(template.FuncMap{
		"add": add,
		"sub": sub,
	}).ParseFS(content, "templates/*.goml"))

	return t, nil
}
