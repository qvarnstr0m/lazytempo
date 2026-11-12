package config

import "testing"

func TestRequireAccountDefaultsTrue(t *testing.T) {
	if !Default().RequireAccount {
		t.Error("RequireAccount should default to true")
	}
}

func TestRequireAccountEnvOverride(t *testing.T) {
	cases := map[string]bool{"false": false, "0": false, "true": true, "1": true}
	for v, want := range cases {
		t.Setenv("LAZYTEMPO_REQUIRE_ACCOUNT", v)
		c := Default()
		c.applyEnv()
		if c.RequireAccount != want {
			t.Errorf("env %q: RequireAccount = %v, want %v", v, c.RequireAccount, want)
		}
	}

	// An unparseable value leaves the default untouched.
	t.Setenv("LAZYTEMPO_REQUIRE_ACCOUNT", "maybe")
	c := Default()
	c.applyEnv()
	if !c.RequireAccount {
		t.Error("unparseable value should keep the default (true)")
	}
}
