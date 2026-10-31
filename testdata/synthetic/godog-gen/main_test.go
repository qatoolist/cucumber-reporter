// Package godoggen produces testdata/synthetic/godog-v0.16.0.json: real Godog v0.16.0 cucumber
// formatter output for invented features, covering all six step statuses, a data table, a doc string,
// a scenario outline and text, JSON, HTML and binary attachments. It is a separate module so that the
// reporter module itself keeps no dependencies. Regenerate with `go test ./...` in this directory and
// copy cucumber.json over the fixture.
package godoggen

import (
	"context"
	"errors"
	"os"
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
