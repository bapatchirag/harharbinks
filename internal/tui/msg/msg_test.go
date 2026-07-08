package msg

import "testing"

func TestLevelString(t *testing.T) {
	cases := []struct {
		level Level
		want  string
	}{
		{Info, "info"},
		{Success, "success"},
		{Warning, "warning"},
		{Error, "error"},
		{Level(99), "info"},
	}
	for _, c := range cases {
		if got := c.level.String(); got != c.want {
			t.Errorf("Level(%d).String() = %q, want %q", c.level, got, c.want)
		}
	}
}
