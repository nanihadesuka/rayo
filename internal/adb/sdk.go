package adb

import (
	"os"
	"path/filepath"
)

// SDKFile finds something in the Android SDK; "" if it isn't there.
func SDKFile(path ...string) string {
	home, _ := os.UserHomeDir()
	for _, sdk := range []string{
		os.Getenv("ANDROID_HOME"),
		os.Getenv("ANDROID_SDK_ROOT"),
		filepath.Join(home, "AppData", "Local", "Android", "Sdk"),
		filepath.Join(home, "Library", "Android", "sdk"),
		filepath.Join(home, "Android", "Sdk"),
	} {
		p := filepath.Join(append([]string{sdk}, path...)...)
		if _, err := os.Stat(p); sdk != "" && err == nil {
			return p
		}
	}
	return ""
}
