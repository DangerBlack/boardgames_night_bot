package telegram

import (
	"strings"
	"testing"
	"time"
)

func strPtr(s string) *string { return &s }
func timePtr(t time.Time) *time.Time { return &t }

func TestParseCreateArgs(t *testing.T) {
	tests := []struct {
		name            string
		args            []string
		fullText        string
		wantName        string
		wantLocation    *string
		wantAllowJoin   bool
		wantStartsAtNil bool
		wantStartsAt    *time.Time
	}{
		{
			name:            "location on last line (no trailing newline)",
			args:            []string{"👥", "SPASSOLA"},
			fullText:        "/create 👥 SPASSOLA\n📍grottaminchia",
			wantName:        "SPASSOLA",
			wantLocation:    strPtr("grottaminchia"),
			wantAllowJoin:   true,
			wantStartsAtNil: true,
		},
		{
			name:            "location on middle line",
			args:            []string{"👥", "SPASSOLA"},
			fullText:        "/create 👥 SPASSOLA\n📍grottaminchia\nsome extra",
			wantName:        "SPASSOLA",
			wantLocation:    strPtr("grottaminchia"),
			wantAllowJoin:   true,
			wantStartsAtNil: true,
		},
		{
			name:            "no location",
			args:            []string{"👥", "SPASSOLA"},
			fullText:        "/create 👥 SPASSOLA",
			wantName:        "SPASSOLA",
			wantLocation:    nil,
			wantAllowJoin:   true,
			wantStartsAtNil: true,
		},
		{
			name:            "no general join flag",
			args:            []string{"SPASSOLA"},
			fullText:        "/create SPASSOLA\n📍grottaminchia",
			wantName:        "SPASSOLA",
			wantLocation:    strPtr("grottaminchia"),
			wantAllowJoin:   false,
			wantStartsAtNil: true,
		},
		{
			name:            "ISO date format YYYY-MM-DD",
			args:            []string{"SPASSOLA", "2025-06-01", "20:00"},
			fullText:        "/create SPASSOLA 2025-06-01 20:00\n📍grottaminchia",
			wantName:        "SPASSOLA",
			wantLocation:    strPtr("grottaminchia"),
			wantAllowJoin:   false,
			wantStartsAtNil: false,
			wantStartsAt:    timePtr(time.Date(2025, 6, 1, 20, 0, 0, 0, time.UTC)),
		},
		{
			name:            "DD-MM-YYYY date format with @bot suffix",
			args:            []string{"👥", "01-04-2026", "20:30"},
			fullText:        "/create@bg_night_bot 👥 01-04-2026 20:30",
			wantName:        "",
			wantLocation:    nil,
			wantAllowJoin:   true,
			wantStartsAtNil: false,
			wantStartsAt:    timePtr(time.Date(2026, 4, 1, 20, 30, 0, 0, time.UTC)),
		},
		{
			name:            "DD-MM-YYYY with name — name must not contain date or 👥",
			args:            []string{"👥", "SPASSOLA", "01-04-2026", "20:30"},
			fullText:        "/create@bg_night_bot 👥 SPASSOLA 01-04-2026 20:30",
			wantName:        "SPASSOLA",
			wantLocation:    nil,
			wantAllowJoin:   true,
			wantStartsAtNil: false,
			wantStartsAt:    timePtr(time.Date(2026, 4, 1, 20, 30, 0, 0, time.UTC)),
		},
		{
			name:            "ISO date on last line",
			args:            []string{"👥", "SPASSOLA"},
			fullText:        "/create 👥 SPASSOLA\n📍grottaminchia\n2023-12-31 20:30",
			wantName:        "SPASSOLA",
			wantLocation:    strPtr("grottaminchia"),
			wantAllowJoin:   true,
			wantStartsAtNil: false,
			wantStartsAt:    timePtr(time.Date(2023, 12, 31, 20, 30, 0, 0, time.UTC)),
		},
		{
			name:            "location with spaces",
			args:            []string{"SPASSOLA"},
			fullText:        "/create SPASSOLA\n📍Via Roma 12, Milano",
			wantName:        "SPASSOLA",
			wantLocation:    strPtr("Via Roma 12, Milano"),
			wantAllowJoin:   false,
			wantStartsAtNil: true,
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

			if tc.wantStartsAtNil && gotStartsAt != nil {
				t.Errorf("startsAt: got %v, want nil", *gotStartsAt)
			}
			if !tc.wantStartsAtNil && gotStartsAt == nil {
				t.Errorf("startsAt: got nil, want non-nil")
			}
			if tc.wantStartsAt != nil && gotStartsAt != nil && !gotStartsAt.Equal(*tc.wantStartsAt) {
				t.Errorf("startsAt: got %v, want %v", *gotStartsAt, *tc.wantStartsAt)
			}
		})
	}
}

func TestParseCreateArgsNameDoesNotContainMetadata(t *testing.T) {
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
