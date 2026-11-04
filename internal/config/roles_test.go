package config

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestRoleDefaultsAndOverrides(t *testing.T) {
	c, err := Parse([]byte("kit_version = 'v3'\n[roles]\nread_only_tools = ['Read']\n[roles.senior]\nkind = 'work'\nagent = 'devkit-senior'\n[planning]\nrole = 'planner'\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(c.RoleSettings()) != 6 || c.RoleSettings()["senior"] != (Role{Kind: "work", Agent: "devkit-senior"}) || !reflect.DeepEqual(c.ReadOnlyTools(), []string{"Read"}) || c.PlanningRole() != "planner" || c.PlanCheckSettings().Runner != "judge" {
		t.Fatalf("unexpected roles: %+v", c)
	}
}

func TestInvalidRoleNamesAndKinds(t *testing.T) {
	for _, body := range []string{
		"[roles.other]\nkind='work'\nagent='outside'",
		"[roles.other]\nkind='judge'\nagent='devkit-worker'",
		"[roles.other]\nkind='human'",
		"[roles.other]\nkind='judge'",
		"[roles.worker]\nscope='checkout'",
		"[planning]\nrole='worker'",
		"[plan_check]\nrunner='planner'",
		"[judges.judge]\nexecutable='judge'\ntimeout_seconds=1\noutput_schema='schema.json'",
	} {
		t.Run(body, func(t *testing.T) {
			if _, err := Parse([]byte("kit_version='v3'\n" + body)); err == nil {
				t.Fatal("accepted invalid role configuration")
			}
		})
	}
}

func TestOutsideSourcesAreReadOnly(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	for _, writable := range []string{"", "\nwritable=false", "\nwritable=true"} {
		c, err := Parse([]byte(fmt.Sprintf("kit_version='v3'\n[sources.extra]\npath=%q%s", outside, writable)))
		if err != nil {
			t.Fatal(err)
		}
		err = c.ValidateSourcePaths(root)
		if writable == "\nwritable=true" {
			if err == nil || !strings.Contains(err.Error(), "sources.extra.writable must be false") || !strings.Contains(err.Error(), "outside the repository are read-only") {
				t.Fatalf("outside write error = %v", err)
			}
		} else if err != nil || c.ValidateImprovementDestination("extra") == nil {
			t.Fatalf("read-only outside source = %v", err)
		}
	}
	c, err := Parse([]byte(fmt.Sprintf("kit_version='v3'\n[sources.extra]\npath=%q\nwritable=true", filepath.Join(root, "extra"))))
	if err != nil || c.ValidateSourcePaths(root) != nil {
		t.Fatalf("inside absolute source rejected: %v", err)
	}
}
