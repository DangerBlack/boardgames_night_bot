package telegram

import (
	"strings"
	"testing"
	"time"
)

// parseCreateArgs mirrors the parsing logic inside CreateGame so it can be
// unit-tested without a live bot context.
func parseCreateArgs(fullText string) (eventName string, location *string, startsAt *time.Time, allowGeneralJoin bool) {
	// Strip the /create (or /create@bot_name) command prefix before processing args
	text := strings.TrimSpace(fullText)
	if idx := strings.Index(text, " "); idx != -1 {
		text = strings.TrimSpace(text[idx+1:])
	} else {
		text = ""
	}

	// Remove location line from event name
	eventName = locationRegex.ReplaceAllString(text, "")
	// Remove datetime from event name
	eventName = dateTimeRegex.ReplaceAllString(eventName, "")
	// Remove general join symbol from event name
	eventName = strings.ReplaceAll(eventName, "👥", "")
	eventName = strings.TrimSpace(eventName)

	if dateTimeStr := dateTimeRegex.FindString(fullText); dateTimeStr != "" {
		var parsed time.Time
		var parseErr error
		for _, layout := range []string{"02-01-2006 15:04", "2006-01-02 15:04"} {
			parsed, parseErr = time.ParseInLocation(layout, dateTimeStr, time.UTC)
			if parseErr == nil {
				break
			}
		}
		if parseErr == nil {
			startsAt = &parsed
		}
	}

	if locationStr := locationRegex.FindStringSubmatch(fullText); len(locationStr) > 1 {
		loc := strings.TrimSpace(locationStr[1])
		location = &loc
	}

	allowGeneralJoin = strings.Contains(fullText, "👥")
	return
}

func strPtr(s string) *string { return &s }

func TestParseCreateArgs(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		wantName        string
		wantLocation    *string
		wantAllowJoin   bool
		wantStartsAtNil bool
		wantStartsAt    *time.Time
	}{
		{
			name:            "location on last line (no trailing newline)",
			input:           "/create 👥 SPASSOLA\n📍grottaminchia",
			wantName:        "SPASSOLA",
			wantLocation:    strPtr("grottaminchia"),
			wantAllowJoin:   true,
			wantStartsAtNil: true,
		},
		{
			name:            "location on middle line (trailing newline present)",
			input:           "/create 👥 SPASSOLA\n📍grottaminchia\nsome extra",
			wantName:        "SPASSOLA\nsome extra",
			wantLocation:    strPtr("grottaminchia"),
			wantAllowJoin:   true,
			wantStartsAtNil: true,
		},
		{
			name:            "no location",
			input:           "/create 👥 SPASSOLA",
			wantName:        "SPASSOLA",
			wantLocation:    nil,
			wantAllowJoin:   true,
			wantStartsAtNil: true,
		},
		{
			name:            "no general join flag",
			input:           "/create SPASSOLA\n📍grottaminchia",
			wantName:        "SPASSOLA",
			wantLocation:    strPtr("grottaminchia"),
			wantAllowJoin:   false,
			wantStartsAtNil: true,
		},
		{
			name:            "ISO date format YYYY-MM-DD with location",
			input:           "/create SPASSOLA 2025-06-01 20:00\n📍grottaminchia",
			wantName:        "SPASSOLA",
			wantLocation:    strPtr("grottaminchia"),
			wantAllowJoin:   false,
			wantStartsAtNil: false,
			wantStartsAt:    timePtr(time.Date(2025, 6, 1, 20, 0, 0, 0, time.UTC)),
		},
		{
			name:            "DD-MM-YYYY date format with @bot suffix",
			input:           "/create@bg_night_bot 👥 01-04-2026 20:30",
			wantName:        "",
			wantLocation:    nil,
			wantAllowJoin:   true,
			wantStartsAtNil: false,
			wantStartsAt:    timePtr(time.Date(2026, 4, 1, 20, 30, 0, 0, time.UTC)),
		},
		{
			name:            "ISO date on last line",
			input:           "/create 👥 SPASSOLA\n📍grottaminchia\n2023-12-31 20:30",
			wantName:        "SPASSOLA",
			wantLocation:    strPtr("grottaminchia"),
			wantAllowJoin:   true,
			wantStartsAtNil: false,
			wantStartsAt:    timePtr(time.Date(2023, 12, 31, 20, 30, 0, 0, time.UTC)),
		},
		{
			name:            "location with spaces",
			input:           "/create SPASSOLA\n📍Via Roma 12, Milano",
			wantName:        "SPASSOLA",
			wantLocation:    strPtr("Via Roma 12, Milano"),
			wantAllowJoin:   false,
			wantStartsAtNil: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotName, gotLocation, gotStartsAt, gotAllowJoin := parseCreateArgs(tc.input)

			if gotName != tc.wantName {
				t.Errorf("eventName: got %q, want %q", gotName, tc.wantName)
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

func timePtr(t time.Time) *time.Time { return &t }
