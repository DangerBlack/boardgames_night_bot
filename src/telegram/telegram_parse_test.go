package telegram

import (
	"strings"
	"testing"
	"time"
)

func strPtr(s string) *string    { return &s }
func timePtr(t time.Time) *time.Time { return &t }

var (
	isoTime   = timePtr(time.Date(2025, 6, 1, 20, 0, 0, 0, time.UTC))
	ddmmTime  = timePtr(time.Date(2026, 4, 1, 20, 30, 0, 0, time.UTC))
)

// TestParseCreateCommandPermutations covers all 16 combinations of
// join(👥) × name × location × time, plus both time formats where applicable.
func TestParseCreateCommandPermutations(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		fullText      string
		wantName      string
		wantLocation  *string
		wantAllowJoin bool
		wantStartsAt  *time.Time
	}{
		// ── no join ──────────────────────────────────────────────────────────
		{
			name:     "name only",
			args:     []string{"SPASSOLA"},
			fullText: "/create SPASSOLA",
			wantName: "SPASSOLA",
		},
		{
			name:         "name + location",
			args:         []string{"SPASSOLA"},
			fullText:     "/create SPASSOLA\n📍Via Roma",
			wantName:     "SPASSOLA",
			wantLocation: strPtr("Via Roma"),
		},
		{
			name:         "name + ISO time",
			args:         []string{"SPASSOLA", "2025-06-01", "20:00"},
			fullText:     "/create SPASSOLA 2025-06-01 20:00",
			wantName:     "SPASSOLA",
			wantStartsAt: isoTime,
		},
		{
			name:         "name + DD-MM-YYYY time",
			args:         []string{"SPASSOLA", "01-04-2026", "20:30"},
			fullText:     "/create SPASSOLA 01-04-2026 20:30",
			wantName:     "SPASSOLA",
			wantStartsAt: ddmmTime,
		},
		{
			name:         "name + location + ISO time",
			args:         []string{"SPASSOLA", "2025-06-01", "20:00"},
			fullText:     "/create SPASSOLA 2025-06-01 20:00\n📍Via Roma",
			wantName:     "SPASSOLA",
			wantLocation: strPtr("Via Roma"),
			wantStartsAt: isoTime,
		},
		{
			name:         "name + location + DD-MM-YYYY time",
			args:         []string{"SPASSOLA", "01-04-2026", "20:30"},
			fullText:     "/create SPASSOLA 01-04-2026 20:30\n📍Via Roma",
			wantName:     "SPASSOLA",
			wantLocation: strPtr("Via Roma"),
			wantStartsAt: ddmmTime,
		},
		{
			name:         "location only (no name)",
			args:         []string{},
			fullText:     "/create\n📍Via Roma",
			wantName:     "",
			wantLocation: strPtr("Via Roma"),
		},
		{
			name:         "time only (no name)",
			args:         []string{"2025-06-01", "20:00"},
			fullText:     "/create 2025-06-01 20:00",
			wantName:     "",
			wantStartsAt: isoTime,
		},

		// ── with join (👥) ────────────────────────────────────────────────────
		{
			name:          "join + name only",
			args:          []string{"👥", "SPASSOLA"},
			fullText:      "/create 👥 SPASSOLA",
			wantName:      "SPASSOLA",
			wantAllowJoin: true,
		},
		{
			name:          "join + name + location",
			args:          []string{"👥", "SPASSOLA"},
			fullText:      "/create 👥 SPASSOLA\n📍Via Roma",
			wantName:      "SPASSOLA",
			wantLocation:  strPtr("Via Roma"),
			wantAllowJoin: true,
		},
		{
			name:          "join + name + ISO time",
			args:          []string{"👥", "SPASSOLA", "2025-06-01", "20:00"},
			fullText:      "/create 👥 SPASSOLA 2025-06-01 20:00",
			wantName:      "SPASSOLA",
			wantAllowJoin: true,
			wantStartsAt:  isoTime,
		},
		{
			name:          "join + name + DD-MM-YYYY time",
			args:          []string{"👥", "SPASSOLA", "01-04-2026", "20:30"},
			fullText:      "/create 👥 SPASSOLA 01-04-2026 20:30",
			wantName:      "SPASSOLA",
			wantAllowJoin: true,
			wantStartsAt:  ddmmTime,
		},
		{
			name:          "join + name + location + ISO time",
			args:          []string{"👥", "SPASSOLA", "2025-06-01", "20:00"},
			fullText:      "/create 👥 SPASSOLA 2025-06-01 20:00\n📍Via Roma",
			wantName:      "SPASSOLA",
			wantLocation:  strPtr("Via Roma"),
			wantAllowJoin: true,
			wantStartsAt:  isoTime,
		},
		{
			name:          "join + name + location + DD-MM-YYYY time",
			args:          []string{"👥", "SPASSOLA", "01-04-2026", "20:30"},
			fullText:      "/create 👥 SPASSOLA 01-04-2026 20:30\n📍Via Roma",
			wantName:      "SPASSOLA",
			wantLocation:  strPtr("Via Roma"),
			wantAllowJoin: true,
			wantStartsAt:  ddmmTime,
		},
		{
			name:          "join + location only (no name)",
			args:          []string{"👥"},
			fullText:      "/create 👥\n📍Via Roma",
			wantName:      "",
			wantLocation:  strPtr("Via Roma"),
			wantAllowJoin: true,
		},
		{
			name:          "join + ISO time only (no name)",
			args:          []string{"👥", "2025-06-01", "20:00"},
			fullText:      "/create 👥 2025-06-01 20:00",
			wantName:      "",
			wantAllowJoin: true,
			wantStartsAt:  isoTime,
		},
		// ── edge cases ────────────────────────────────────────────────────────
		{
			name:         "location on last line (no trailing newline)",
			args:         []string{"SPASSOLA"},
			fullText:     "/create SPASSOLA\n📍grottaminchia",
			wantName:     "SPASSOLA",
			wantLocation: strPtr("grottaminchia"),
		},
		{
			name:         "location with spaces",
			args:         []string{"SPASSOLA"},
			fullText:     "/create SPASSOLA\n📍Via Roma 12, Milano",
			wantName:     "SPASSOLA",
			wantLocation: strPtr("Via Roma 12, Milano"),
		},
		{
			name:          "@bot suffix in command",
			args:          []string{"👥", "01-04-2026", "20:30"},
			fullText:      "/create@bg_night_bot 👥 01-04-2026 20:30",
			wantName:      "",
			wantAllowJoin: true,
			wantStartsAt:  ddmmTime,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotName, gotLocation, gotStartsAt, gotAllowJoin := parseCreateCommand(tc.args, tc.fullText, time.UTC)

			if gotName != tc.wantName {
				t.Errorf("name: got %q, want %q", gotName, tc.wantName)
			}
			if tc.wantLocation == nil && gotLocation != nil {
				t.Errorf("location: got %q, want nil", *gotLocation)
			} else if tc.wantLocation != nil && gotLocation == nil {
				t.Errorf("location: got nil, want %q", *tc.wantLocation)
			} else if tc.wantLocation != nil && gotLocation != nil && *gotLocation != *tc.wantLocation {
				t.Errorf("location: got %q, want %q", *gotLocation, *tc.wantLocation)
			}
			if gotAllowJoin != tc.wantAllowJoin {
				t.Errorf("allowGeneralJoin: got %v, want %v", gotAllowJoin, tc.wantAllowJoin)
			}
			if tc.wantStartsAt == nil && gotStartsAt != nil {
				t.Errorf("startsAt: got %v, want nil", *gotStartsAt)
			} else if tc.wantStartsAt != nil && gotStartsAt == nil {
				t.Errorf("startsAt: got nil, want %v", *tc.wantStartsAt)
			} else if tc.wantStartsAt != nil && gotStartsAt != nil && !gotStartsAt.Equal(*tc.wantStartsAt) {
				t.Errorf("startsAt: got %v, want %v", *gotStartsAt, *tc.wantStartsAt)
			}
		})
	}
}

func TestParseCreateCommandNameDoesNotContainMetadata(t *testing.T) {
	// Regression: event name was including the date/location/👥 markers
	args := []string{"👥", "SPASSOLA\n📍grottaminchia\n2023-12-31", "20:30"}
	fullText := "/create 👥 SPASSOLA\n📍grottaminchia\n2023-12-31 20:30"

	name, _, _, _ := parseCreateCommand(args, fullText, time.UTC)

	if strings.Contains(name, "📍") {
		t.Errorf("name contains location marker: %q", name)
	}
	if strings.Contains(name, "👥") {
		t.Errorf("name contains join marker: %q", name)
	}
	if strings.Contains(name, "2023") {
		t.Errorf("name contains date: %q", name)
	}
}
