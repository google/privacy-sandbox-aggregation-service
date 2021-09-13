package utils

import (
	"fmt"
	"testing"
)

func TestParsePubSubResourceName(t *testing.T) {
	type parseResult struct {
		Input, Project, Name string
		ErrStr               string
	}
	for _, want := range []parseResult{
		{
			Input:   "projects/myproject/topics/mytopic",
			Project: "myproject",
			Name:    "mytopic",
		},
		{
			Input:   "projects/myproject/topics",
			Project: "",
			Name:    "",
			ErrStr:  fmt.Sprintf("expect format %s, got %s", "projects/project-identifier/collection/relative-name", "projects/myproject/topics"),
		},
		{
			Input:   "projects/myproject/foo/mytopic",
			Project: "",
			Name:    "",
			ErrStr:  fmt.Sprintf("expect format %s, got %s", "projects/project-identifier/collection/relative-name", "projects/myproject/foo/mytopic"),
		},
	} {
		project, name, err := ParsePubSubResourceName(want.Input)
		var errStr string
		if err != nil {
			errStr = err.Error()
		}
		if want.ErrStr != errStr {
			t.Errorf("expect error message %s, got %s", want.ErrStr, errStr)
		}

		if project != want.Project || name != want.Name {
			t.Errorf("want project %q and name %q, got %q, and %q", want.Project, want.Name, project, name)
		}
	}
}
