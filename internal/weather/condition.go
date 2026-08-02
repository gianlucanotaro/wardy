package weather

// conditionFromWMOCode maps Open-Meteo's WMO weather codes down to a small
// set of coarse, human-readable conditions — plenty for "should I suggest
// a coat" without needing every code's exact nuance.
func conditionFromWMOCode(code int) string {
	switch {
	case code == 0:
		return "clear"
	case code >= 1 && code <= 3:
		return "cloudy"
	case code == 45 || code == 48:
		return "fog"
	case code >= 51 && code <= 57:
		return "drizzle"
	case code >= 61 && code <= 67:
		return "rain"
	case code >= 71 && code <= 77:
		return "snow"
	case code >= 80 && code <= 82:
		return "showers"
	case code >= 85 && code <= 86:
		return "snow"
	case code >= 95 && code <= 99:
		return "thunderstorm"
	default:
		return "unknown"
	}
}
