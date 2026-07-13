package output

import "testing"

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input uint64
		want  string
	}{
		{0, "0B"},
		{512, "512B"},
		{1024, "1.0Ki"},
		{1536, "1.5Ki"},
		{1048576, "1.0Mi"},
		{1073741824, "1.0Gi"},
		{1099511627776, "1.0Ti"},
		{2684354560, "2.5Gi"},
	}
	for _, tt := range tests {
		got := FormatBytes(tt.input)
		if got != tt.want {
			t.Errorf("FormatBytes(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFormatUptime(t *testing.T) {
	tests := []struct {
		input uint64
		want  string
	}{
		{0, "0s"},
		{45, "45s"},
		{90, "1m 30s"},
		{3600, "1h 0m"},
		{3661, "1h 1m"},
		{86400, "1d 0m"},
		{90061, "1d 1h 1m"},
		{172800, "2d 0m"},
	}
	for _, tt := range tests {
		got := FormatUptime(tt.input)
		if got != tt.want {
			t.Errorf("FormatUptime(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFormatPercent(t *testing.T) {
	tests := []struct {
		input float64
		want  string
	}{
		{0.0, "0.0%"},
		{0.5, "50.0%"},
		{1.0, "100.0%"},
		{0.123, "12.3%"},
		{0.999, "99.9%"},
	}
	for _, tt := range tests {
		got := FormatPercent(tt.input)
		if got != tt.want {
			t.Errorf("FormatPercent(%f) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
