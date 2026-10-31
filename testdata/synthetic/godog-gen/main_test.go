// Package godoggen produces the real Godog v0.16.0 cucumber formatter output in testdata/synthetic
// for invented features. TestFeatures writes cucumber.json (copy it to godog-v0.16.0.json): all six
// step statuses, a data table, a doc string, a scenario outline and text, JSON, HTML and binary
// attachments. TestStepless writes cucumber-stepless.json (copy it to godog-v0.16.0-stepless.json) and
// Godog's own summary of it. This is a separate module so that the reporter module keeps no
// dependencies. Regenerate with `go test ./...` in this directory.
package godoggen

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/cucumber/godog"
)

func steps(sc *godog.ScenarioContext) {
	sc.Step(`^a draft order with these lines:$`, func(ctx context.Context, t *godog.Table) (context.Context, error) {
		ctx = godog.Attach(ctx, godog.Attachment{Body: []byte("lines accepted\n"), FileName: "lines.txt", MediaType: "text/plain"})
		return ctx, nil
	})
	sc.Step(`^the order is confirmed$`, func(ctx context.Context) (context.Context, error) {
		ctx = godog.Attach(ctx,
			godog.Attachment{Body: []byte(`{"order":"ORD-1","state":"confirmed"}`), FileName: "order.json", MediaType: "application/json"},
			godog.Attachment{Body: []byte(`<b>bold</b>`), FileName: "page.html", MediaType: "text/html"},
			godog.Attachment{Body: []byte{0x89, 'P', 'N', 'G', 0, 1, 2}, FileName: "pixel.png", MediaType: "image/png"})
		return ctx, nil
	})
	sc.Step(`^the confirmation message is:$`, func(*godog.DocString) error { return nil })
	sc.Step(`^a draft order with quantity (-?\d+)$`, func(int) error { return nil })
	sc.Step(`^the order is rejected$`, func() error { return errors.New("expected rejection, got confirmation") })
	sc.Step(`^an invoice exists$`, func() error { return nil })
	sc.Step(`^the invoice is sent$`, func() error { return godog.ErrPending })
	sc.Step(`^the customer receives it$`, func() error { return nil })
	sc.Step(`^an ambiguous step$`, func() error { return nil })
	sc.Step(`^an ambiguous (step)$`, func(string) error { return nil })
}

func TestFeatures(t *testing.T) {
	f, _ := os.Create("cucumber.json")
	defer f.Close()
	s := godog.TestSuite{ScenarioInitializer: steps, Options: &godog.Options{Format: "cucumber", Paths: []string{"features"}, Output: f, Strict: true}}
	s.Run()
}

// TestStepless produces godog-v0.16.0-stepless.json: a scenario without steps, which Godog writes
// without a "steps" key and counts as undefined, beside a passed and a pending scenario.
func TestStepless(t *testing.T) {
	for _, format := range []string{"cucumber:cucumber-stepless.json", "pretty:summary-stepless.txt"} {
		name, path, _ := strings.Cut(format, ":")
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		s := godog.TestSuite{ScenarioInitializer: steps, Options: &godog.Options{Format: name, Paths: []string{"features-stepless"}, Output: f, Strict: true, NoColors: true}}
		s.Run()
		_ = f.Close()
	}
}
