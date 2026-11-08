package main

import (
	"regexp"
	"runtime/debug"
	"strings"
)

// developmentVersion is the next release; RELEASE.md requires it to match the tag.
var developmentVersion = "0.3.0"

var pseudoVersion = regexp.MustCompile(`(?:^|[-.])\d{14}-([0-9a-f]{12})(?:\+dirty)?$`)

func versionFromModule(module string) string {
	if m := pseudoVersion.FindStringSubmatch(module); m != nil {
		return strings.TrimSuffix(developmentVersion, "-dev") + "-dev (" + m[1] + ")"
	}
	if module == "" || module == "(devel)" || strings.HasSuffix(module, "+dirty") {
		return developmentVersion
	}
	return strings.TrimPrefix(module, "v")
}

func version() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		result := versionFromModule(info.Main.Version)
		if strings.HasSuffix(result, "-dev") {
			for _, setting := range info.Settings {
				if setting.Key == "vcs.revision" && setting.Value != "" {
					return result + " (" + setting.Value[:min(12, len(setting.Value))] + ")"
				}
			}
		}
		return result
	}
	return developmentVersion
}
