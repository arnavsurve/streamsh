package streamsh

import "testing"

func TestStripANSI(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain text", "hello world", "hello world"},
		{"color codes", "\x1b[31mred\x1b[0m", "red"},
		{"bold", "\x1b[1mbold\x1b[0m", "bold"},
		{"cursor movement", "\x1b[2J\x1b[H", ""},
		{"OSC title set", "\x1b]2;my title\x07rest", "rest"},
		{"OSC with ST", "\x1b]0;title\x1b\\rest", "rest"},
		{"mixed", "\x1b[32m± \x1b[0m\x1b[36m~/dev\x1b[0m", "± ~/dev"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripANSI(tt.input)
			if got != tt.want {
				t.Errorf("stripANSI(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
