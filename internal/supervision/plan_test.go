// SPDX-License-Identifier: MPL-2.0

package supervision

import (
	"encoding/xml"
	"io"
	"strings"
	"testing"
)

func validOptions() Options {
	return Options{Platform: "darwin", Binary: "/opt/Builder & Co/agent-commons", State: "/private/role state", SearchPath: "/usr/bin:/opt/bin"}
}

func TestLaunchdPlan(t *testing.T) {
	p, err := Render(validOptions())
	if err != nil {
		t.Fatal(err)
	}
	d := xml.NewDecoder(strings.NewReader(p.Content))
	for {
		_, err = d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, required := range []string{"Builder &amp; Co", "SuccessfulExit", "ThrottleInterval", "<integer>63</integer>"} {
		if !strings.Contains(p.Content, required) {
			t.Fatalf("missing %s", required)
		}
	}
	if !strings.HasSuffix(p.Filename, ".plist") {
		t.Fatal(p.Filename)
	}
}

func TestSystemdPlanEscapesSpecifierAndDisablesExpansion(t *testing.T) {
	o := validOptions()
	o.Platform = "linux"
	o.State = `/private/100% $STATE "name"`
	p, err := Render(o)
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{`ExecStart=:"/opt/Builder & Co/agent-commons"`, `100%% $STATE \"name\"`, "KillMode=control-group", "UMask=0077"} {
		if !strings.Contains(p.Content, required) {
			t.Fatalf("missing %s in %s", required, p.Content)
		}
	}
}

func TestRejectUnsafeOptions(t *testing.T) {
	for _, mutate := range []func(*Options){
		func(o *Options) { o.State = "/" }, func(o *Options) { o.State = "relative" },
		func(o *Options) { o.Binary = "/bin/tool\nInjected=yes" },
		func(o *Options) { o.SearchPath = "/bin:." }, func(o *Options) { o.SearchPath = "/bin:" },
		func(o *Options) { o.Platform = "windows" },
	} {
		o := validOptions()
		mutate(&o)
		if _, err := Render(o); err == nil {
			t.Fatalf("accepted %+v", o)
		}
	}
}

func TestStateDeterminesStableLabel(t *testing.T) {
	o := validOptions()
	a, _ := Render(o)
	o.Binary = "/new/agent-commons"
	b, _ := Render(o)
	if a.Label != b.Label {
		t.Fatal("upgrade changed label")
	}
	o.State = "/different/state"
	c, _ := Render(o)
	if a.Label == c.Label {
		t.Fatal("state collision")
	}
}
