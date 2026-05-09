package browser

import "strings"

type Profile struct {
	Raw            string
	BrowserName    string
	BrowserVersion string
	OSName         string
	DeviceType     string
	Key            string
}

func Parse(userAgent string) Profile {
	lowerUA := strings.ToLower(strings.TrimSpace(userAgent))
	profile := Profile{
		Raw:        userAgent,
		OSName:     detectOS(lowerUA),
		DeviceType: detectDeviceType(lowerUA),
	}

	profile.BrowserName, profile.BrowserVersion = detectBrowser(lowerUA)
	if profile.BrowserName != "" {
		osName := profile.OSName
		if osName == "" {
			osName = "unknown"
		}
		deviceType := profile.DeviceType
		if deviceType == "" {
			deviceType = "unknown"
		}
		profile.Key = strings.Join([]string{profile.BrowserName, osName, deviceType}, "|")
	}

	return profile
}

func detectBrowser(ua string) (string, string) {
	switch {
	case strings.Contains(ua, "edg/"):
		return "edge", extractVersion(ua, "edg/")
	case strings.Contains(ua, "opr/"):
		return "opera", extractVersion(ua, "opr/")
	case strings.Contains(ua, "opera/"):
		return "opera", extractVersion(ua, "opera/")
	case strings.Contains(ua, "firefox/"):
		return "firefox", extractVersion(ua, "firefox/")
	case strings.Contains(ua, "chrome/"):
		return "chrome", extractVersion(ua, "chrome/")
	case strings.Contains(ua, "version/") && strings.Contains(ua, "safari/"):
		return "safari", extractVersion(ua, "version/")
	case strings.Contains(ua, "msie "):
		return "ie", extractVersion(ua, "msie ")
	case strings.Contains(ua, "trident/"):
		return "ie", ""
	default:
		return "", ""
	}
}

func detectOS(ua string) string {
	switch {
	case strings.Contains(ua, "iphone"), strings.Contains(ua, "ipad"), strings.Contains(ua, "cpu iphone os"), strings.Contains(ua, "cpu os"):
		return "ios"
	case strings.Contains(ua, "android"):
		return "android"
	case strings.Contains(ua, "windows nt"):
		return "windows"
	case strings.Contains(ua, "mac os x"), strings.Contains(ua, "macintosh"):
		return "macos"
	case strings.Contains(ua, "linux"):
		return "linux"
	default:
		return ""
	}
}

func detectDeviceType(ua string) string {
	switch {
	case strings.Contains(ua, "ipad"), strings.Contains(ua, "tablet"):
		return "tablet"
	case strings.Contains(ua, "android") && !strings.Contains(ua, "mobile"):
		return "tablet"
	case strings.Contains(ua, "iphone"), strings.Contains(ua, "mobile"):
		return "mobile"
	case ua == "":
		return ""
	default:
		return "desktop"
	}
}

func extractVersion(ua, marker string) string {
	index := strings.Index(ua, marker)
	if index < 0 {
		return ""
	}
	value := ua[index+len(marker):]
	if value == "" {
		return ""
	}

	end := len(value)
	for i, r := range value {
		if (r < '0' || r > '9') && r != '.' {
			end = i
			break
		}
	}
	value = value[:end]
	if value == "" {
		return ""
	}

	parts := strings.SplitN(value, ".", 2)
	return parts[0]
}
