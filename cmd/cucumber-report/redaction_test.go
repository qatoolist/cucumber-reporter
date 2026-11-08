package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Exercise the Gherkin language label, MIME aliases and missing type through both
// parsers, then inspect the deliverable rather than just the intermediate model.
func TestDocStringJSONRedactionBothFormats(t *testing.T) {
	config := writeFile(t, `{"redaction":{"jsonKeys":["password"]}}`)
	fallbackConfig := writeFile(t, `{"redaction":{"jsonKeys":["password"],"patterns":["DOCSECRET9"]}}`)
	for _, format := range []string{"json", "messages"} {
		for _, contentType := range []string{"json", "text/json", "", "application/json", "application/problem+json", " JSON ; charset=utf-8"} {
			states := []string{"valid", "malformed", "empty"}
			if contentType == "" {
				states = append(states, "log", "template", "markdown", "timestamp")
			}
			for _, state := range states {
				t.Run(format+"/"+contentType+"/"+state, func(t *testing.T) {
					body := `{"password":"DOCSECRET9","public":1}`
					if state == "malformed" {
						body = strings.TrimSuffix(body, "}")
					}
					switch state {
					case "empty":
						body = ""
					case "log":
						body = "[INFO] user logged in"
					case "template":
						body = "{{x}} logs in"
					case "markdown":
						body = "[docs](https://example.test)"
					case "timestamp":
						body = "[2026-11-06 10:00] started"
					}
					docJSON, err := json.Marshal(map[string]any{"content_type": contentType, "value": body})
					if err != nil {
						t.Fatal(err)
					}
					input := fmt.Sprintf(`[{"uri":"d.feature","elements":[{"name":"s","line":3,"steps":[{"name":"x","result":{"status":"passed"},"doc_string":%s}]}]}]`, docJSON)
					if format == "messages" {
						docJSON, err = json.Marshal(map[string]any{"mediaType": contentType, "content": body})
						if err != nil {
							t.Fatal(err)
						}
						input = fmt.Sprintf(`{"pickle":{"id":"p","uri":"d.feature","name":"s","steps":[{"id":"ps","text":"x","argument":{"docString":%s}}]}}
{"testCase":{"id":"tc","pickleId":"p","testSteps":[{"id":"ts","pickleStepId":"ps"}]}}
{"testCaseStarted":{"id":"a","testCaseId":"tc","attempt":0,"timestamp":{"seconds":"1","nanos":0}}}
{"testStepFinished":{"testCaseStartedId":"a","testStepId":"ts","testStepResult":{"status":"PASSED","duration":{"seconds":"0","nanos":1}}}}
{"testCaseFinished":{"testCaseStartedId":"a","timestamp":{"seconds":"2","nanos":0}}}
{"testRunFinished":{"success":true}}`, docJSON)
					}
					file := writeFile(t, input)
					parent := t.TempDir()
					policy := config
					if contentType == "" && state == "malformed" {
						policy = fallbackConfig
					}
					stdout, stderr, code := command(t, "-enhanced", "-format", format, "-bundle", "-run-config", policy, "-o", parent, file)
					if state == "malformed" && contentType != "" {
						if code != 1 || stdout != "" || !strings.Contains(stderr, "feature[0].elements[0].steps[0].doc_string") || strings.Contains(stderr, "DOCSECRET9") {
							t.Fatalf("unsafe or missing refusal: %d %s %s", code, stdout, stderr)
						}
						files, readErr := os.ReadDir(parent)
						if readErr != nil || len(files) != 0 {
							t.Fatalf("failed policy wrote output: %v %v", files, readErr)
						}
						return
					}
					if code != 0 {
						t.Fatalf("exit %d: %s", code, stderr)
					}
					index := reportPath(t, stdout)
					html, readErr := os.ReadFile(index)
					if readErr != nil {
						t.Fatal(readErr)
					}
					if bytes.Contains(html, []byte("DOCSECRET9")) || (state == "valid" && !bytes.Contains(html, []byte("[REDACTED]"))) {
						t.Fatal("doc string not masked in HTML")
					}
					if state != "valid" && state != "malformed" && state != "empty" && !bytes.Contains(html, []byte(body)) {
						t.Fatal("untyped text disappeared from HTML")
					}
					archive, zipErr := zip.OpenReader(filepath.Join(filepath.Dir(index), "report.zip"))
					if zipErr != nil {
						t.Fatal(zipErr)
					}
					defer archive.Close()
					for _, entry := range archive.File {
						reader, openErr := entry.Open()
						if openErr != nil {
							t.Fatal(openErr)
						}
						data, readErr := io.ReadAll(reader)
						reader.Close()
						if readErr != nil {
							t.Fatal(readErr)
						}
						if bytes.Contains(data, []byte("DOCSECRET9")) {
							t.Fatalf("secret in ZIP entry %s", entry.Name)
						}
					}
				})
			}
		}
	}
}
