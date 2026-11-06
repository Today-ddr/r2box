package database

import (
	"testing"
	"time"
)

func TestParseSQLiteTime(t *testing.T) {
	cases := []struct {
		input string
		want  time.Time
		ok    bool
	}{
		{"2026-11-06 10:00:00+08:00", time.Date(2026, 11, 6, 2, 0, 0, 0, time.UTC), true},
		{"2026-11-06 10:00:00", time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC), true},
		{"2026-11-06T10:00:00Z", time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC), true},
		{" 2026-11-06 10:00:00 ", time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC), true},
		{"", time.Time{}, false},
		{"not a time", time.Time{}, false},
	}

	for _, c := range cases {
		got, ok := ParseSQLiteTime(c.input)
		if ok != c.ok {
			t.Errorf("ParseSQLiteTime(%q) ok = %v, want %v", c.input, ok, c.ok)
			continue
		}
		if ok && !got.Equal(c.want) {
			t.Errorf("ParseSQLiteTime(%q) = %v, want %v", c.input, got.UTC(), c.want)
		}
	}
}
