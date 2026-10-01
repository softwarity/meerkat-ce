// Package useragent names a browser the way a person reads it - "Chrome -
// macOS" - from its User-Agent string. Shared by the sign-in history, the
// trusted browsers and the session lists, so one browser has one name
// wherever it is shown.
package useragent

import "strings"

// Label is the short name of the browser and system a User-Agent describes.
func Label(ua string) string {
	ua = strings.TrimSpace(ua)
	if ua == "" {
		return "Unknown browser"
	}
	browser := ""
	switch {
	case strings.Contains(ua, "Edg/"):
		browser = "Edge"
	case strings.Contains(ua, "OPR/") || strings.Contains(ua, "Opera"):
		browser = "Opera"
	case strings.Contains(ua, "Chrome/"):
		browser = "Chrome"
	case strings.Contains(ua, "Firefox/"):
		browser = "Firefox"
	case strings.Contains(ua, "Safari/"):
		browser = "Safari"
	}
	osName := ""
	switch {
	case strings.Contains(ua, "iPhone"):
		osName = "iPhone"
	case strings.Contains(ua, "iPad"):
		osName = "iPad"
	case strings.Contains(ua, "Android"):
		osName = "Android"
	case strings.Contains(ua, "Mac OS X"), strings.Contains(ua, "Macintosh"):
		osName = "macOS"
	case strings.Contains(ua, "Windows"):
		osName = "Windows"
	case strings.Contains(ua, "CrOS"):
		osName = "ChromeOS"
	case strings.Contains(ua, "Linux"):
		osName = "Linux"
	}
	switch {
	case browser != "" && osName != "":
		return browser + " - " + osName
	case browser != "":
		return browser
	case osName != "":
		return osName
	}
	if len(ua) > 60 {
		ua = ua[:60]
	}
	return ua
}
