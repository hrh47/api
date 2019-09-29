package template

import (
	"bytes"
	htmltpl "html/template"
	"os"
	"path/filepath"
	"strings"

	"github.com/aymerick/douceur/inliner"
	"gopkg.in/russross/blackfriday.v2"
)

// Message is a renderable message. It is always a constituent of a
// Thread. The Body field accepts markdown. XML is not allowed.
type Message struct {
	Body   string
	Name   string
	FromID string
	ToID   string
}

// Thread is a representation of a renderable email thread. It contains
// a subject, a slice of messages, and a preview. All fields are or
// resolve to strings.
type Thread struct {
	Subject  string
	Messages []Message
	Preview  string
}

type Event struct {
	Name        string
	Address     string
	Time        string
	Description string
	Preview     string
	FromName    string
	MagicLink   string
}

type message struct {
	Body   htmltpl.HTML
	Name   string
	FromID string
	ToID   string
}

type thread struct {
	Subject  string
	Messages []message
	Preview  string
}

type event struct {
	Name        string
	Address     string
	Time        string
	Description htmltpl.HTML
	Preview     string
	FromName    string
	MagicLink   string
}

var templates map[string]*htmltpl.Template

func init() {
	if templates == nil {
		templates = make(map[string]*htmltpl.Template)
	}

	var basePath string
	if strings.HasSuffix(os.Args[0], ".test") {
		basePath = "../templates"
	} else {
		basePath = "./templates"
	}

	layouts, err := filepath.Glob(basePath + "/layouts/*.html")
	if err != nil {
		panic(err)
	}

	includes, err := filepath.Glob(basePath + "/includes/*.html")
	if err != nil {
		panic(err)
	}

	// Generate our templates map from our layouts/ and includes/ directories
	for _, layout := range layouts {
		files := append(includes, layout)
		templates[filepath.Base(layout)] = htmltpl.Must(htmltpl.ParseFiles(files...))
	}

	// Make sure the expected templates are there
	_, ok := templates["thread.html"]
	if !ok {
		panic("template thread.html not loaded")
	}

	_, ok = templates["event.html"]
	if !ok {
		panic("template event.html not loaded")
	}
}

// RenderThread returns the email message rendered to a string.
func RenderThread(data Thread) (string, error) {
	tmpl, _ := templates["thread.html"]

	t := thread{
		Subject:  data.Subject,
		Messages: make([]message, len(data.Messages)),
		Preview:  data.Preview,
	}

	// Render markdown to HTML
	for i := range data.Messages {
		t.Messages[i].Body = htmltpl.HTML(blackfriday.Run([]byte(data.Messages[i].Body)))
		t.Messages[i].Name = data.Messages[i].Name
		t.Messages[i].FromID = data.Messages[i].FromID
		t.Messages[i].ToID = data.Messages[i].ToID
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "base.html", t); err != nil {
		return "", err
	}

	html, err := inliner.Inline(buf.String())
	if err != nil {
		return html, err
	}

	return html, nil
}

func RenderEvent(data Event) (string, error) {
	tmpl, _ := templates["event.html"]

	e := event{
		Name:        data.Name,
		Address:     data.Address,
		Time:        data.Time,
		Description: htmltpl.HTML(blackfriday.Run([]byte(data.Description))),
		FromName:    data.FromName,
		MagicLink:   data.MagicLink,
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "base.html", e); err != nil {
		return "", err
	}

	html, err := inliner.Inline(buf.String())
	if err != nil {
		return html, err
	}

	return html, nil
}
