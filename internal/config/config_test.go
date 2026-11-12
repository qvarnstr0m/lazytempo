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

func TestTimeFormat(t *testing.T) {
	if Default().Hour12() {
		t.Error("default time format should be 24-hour (eu)")
	}
	// normalize snaps unknown values to "eu" and recognises "us" (any case).
	c := Config{TimeFormat: "US"}
	c.normalize()
	if c.TimeFormat != "us" || !c.Hour12() {
		t.Errorf("US should normalize to us/hour12, got %q", c.TimeFormat)
	}
	c = Config{TimeFormat: "nonsense"}
	c.normalize()
	if c.TimeFormat != "eu" || c.Hour12() {
		t.Errorf("unknown should fall back to eu, got %q", c.TimeFormat)
	}
	// env override
	t.Setenv("LAZYTEMPO_TIME_FORMAT", "us")
	c = Default()
	c.applyEnv()
	if !c.Hour12() {
		t.Error("env LAZYTEMPO_TIME_FORMAT=us should enable 12-hour")
	}
}
