package template

import (
	"fmt"
	"strings"
)

// Message is a renderable message. It is always a constituent of a
// Thread. The Body field accepts markdown. XML is not allowed.
type Message struct {
	renderable
	Body   string
	Name   string
	FromID string
	ToID   string
}

// Thread is a representation of a renderable email thread.
type Thread struct {
	renderable
	Subject  string
	Messages []Message
	Preview  string
}

// Event is a representation of a renderable email event.
type Event struct {
	renderable
	Name        string
	Address     string
	Time        string
	Description string
	Preview     string
	FromName    string
	MagicLink   string
}

// Digest is a representation of a renderable email digest.
type Digest struct {
	renderable
	Items   []Thread
	Preview string
	Events  []Event
}

// RenderThread returns a rendered thread email.
func RenderThread(t Thread) (string, string, error) {
	var builder strings.Builder

	for i, m := range t.Messages {
		fmt.Fprintf(&builder, _tplStrMessage, m.Name, m.Body)
		t.Messages[i].RenderMarkdown(t.Messages[i].Body)
	}

	plainText := builder.String()
	preview := getPreview(plainText)

	t.Preview = preview

	html, err := t.RenderHTML("thread.html", t)

	return plainText, html, err
}

// RenderEvent returns a rendered event invitation email.
func RenderEvent(e Event) (string, string, error) {
	e.RenderMarkdown(e.Description)

	var builder strings.Builder
	fmt.Fprintf(&builder, _tplStrEvent,
		e.FromName,
		e.Name,
		e.Address,
		e.Time,
		e.Description)
	plainText := builder.String()
	preview := getPreview(plainText)

	e.Preview = preview

	html, err := e.RenderHTML("event.html", e)

	return plainText, html, err
}

// RenderCancellation returns a rendered event cancellation email.
func RenderCancellation(e Event) (string, string, error) {
	var builder strings.Builder
	fmt.Fprintf(&builder, _tplStrCancellation,
		e.FromName,
		e.Name,
		e.Address,
		e.Time)
	plainText := builder.String()
	preview := getPreview(plainText)

	e.Preview = preview

	html, err := e.RenderHTML("cancellation.html", e)

	return plainText, html, err
}

// RenderDigest returns a rendered digest email.
func RenderDigest(d Digest) (string, string, error) {
	for i := range d.Items {
		for j := range d.Items[i].Messages {
			d.Items[i].Messages[j].RenderMarkdown(d.Items[i].Messages[j].Body)
		}
	}

	plainText := "You have notifications on Convo."
	d.Preview = plainText

	html, err := d.RenderHTML("digest.html", d)

	return plainText, html, err
}

func getPreview(plainText string) string {
	if len(plainText) > 200 {
		return plainText[:200] + "..."
	}

	return plainText
}
