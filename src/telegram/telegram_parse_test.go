package telegram

import (
	"strings"
	"testing"
	"time"
)

// parseCreateArgs mirrors the parsing logic inside CreateGame so it can be
// unit-tested without a live bot context.
func parseCreateArgs(fullText string) (eventName string, location *string, startsAt *time.Time, allowGeneralJoin bool) {
	// Strip the /create command prefix before processing args
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
		layout := "02-01-2006 15:04"
		parsed, err := time.ParseInLocation(layout, dateTimeStr, time.UTC)
		if err == nil {
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
		name             string
		input            string
		wantName         string
		wantLocation     *string
		wantAllowJoin    bool
		wantStartsAtNil  bool
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
			name:            "with datetime and location",
			input:           "/create SPASSOLA 01-06-2025 20:00\n📍grottaminchia",
			wantName:        "SPASSOLA",
			wantLocation:    strPtr("grottaminchia"),
			wantAllowJoin:   false,
			wantStartsAtNil: false,
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
		})
	}
}
