package output

import "fmt"

// FormatBytes formats a byte count into a human-readable string
// using binary units (KiB, MiB, GiB, TiB).
func FormatBytes(b uint64) string {
	const (
		kib = 1024
		mib = kib * 1024
		gib = mib * 1024
		tib = gib * 1024
	)
	switch {
	case b >= tib:
		return fmt.Sprintf("%.1fTi", float64(b)/float64(tib))
	case b >= gib:
		return fmt.Sprintf("%.1fGi", float64(b)/float64(gib))
	case b >= mib:
		return fmt.Sprintf("%.1fMi", float64(b)/float64(mib))
	case b >= kib:
		return fmt.Sprintf("%.1fKi", float64(b)/float64(kib))
	default:
		return fmt.Sprintf("%dB", b)
	}
}

// FormatUptime formats a duration in seconds into a human-readable string
// like "3d 5h 12m" or "45s".
func FormatUptime(seconds uint64) string {
	if seconds == 0 {
		return "0s"
	}

	days := seconds / 86400
	hours := (seconds % 86400) / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60

	switch {
	case days > 0:
		if hours > 0 {
			return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
		}
		return fmt.Sprintf("%dd %dm", days, minutes)
	case hours > 0:
		return fmt.Sprintf("%dh %dm", hours, minutes)
	case minutes > 0:
		return fmt.Sprintf("%dm %ds", minutes, secs)
	default:
		return fmt.Sprintf("%ds", secs)
	}
}

// FormatPercent formats a float64 (0.0–1.0) as a percentage string.
func FormatPercent(ratio float64) string {
	return fmt.Sprintf("%.1f%%", ratio*100)
}
