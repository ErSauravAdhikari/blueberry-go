package blueberry

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

func add(a, b int) int {
	return a + b
}

func sub(a, b int) int {
	return a - b
}

// Format datetime to a readable string
func formatDateTime(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}

// Format Unix timestamp to a readable string
func formatTimestamp(timestamp int64) string {
	return time.Unix(timestamp, 0).Format("2006-01-02 15:04:05")
}

// loadTemplates loads and parses the templates with additional functions
func loadTemplates() (*template.Template, error) {
	t := template.Must(template.New("").Funcs(template.FuncMap{
		"add":             add,
		"sub":             sub,
		"formatDateTime":  formatDateTime,
		"formatTimestamp": formatTimestamp,
	}).ParseFS(content, "templates/*.goml"))

	return t, nil
}
