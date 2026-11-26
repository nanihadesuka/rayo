package flow

import "testing"

func TestBlank(t *testing.T) {
	const bar = "[a.b]\n  id: android:id/decor_content_parent  [0,0][1080,2361]\n    id: android:id/action_bar  [0,142][1080,289]\n      text: Slow  [42,180][157,251]\n"
	const system = "[com.android.systemui]\n  id: com.android.systemui:id/clock | text: 12:00  [0,0][40,20]\n"
	for _, c := range []struct {
		name, screen, app string
		want              bool
	}{
		{"empty content", "[a.b]\n  id: android:id/content  [0,106][540,1212]\n" + system, "a.b", true},
		{"empty content, an action bar", bar + "    id: android:id/content  [0,289][1080,2361]\n  id: android:id/navigationBarBackground  [0,2361][1080,2424]\n" + system, "a.b", true},
		{"empty content, last", "[a.b]\n  id: android:id/content  [0,106][540,1212]", "a.b", true},
		{"something in it", bar + "    id: android:id/content  [0,289][1080,2361]\n      text: Start  [100,246][300,326]\n", "a.b", false},
		{"another app in front", system + "[a.b]\n  id: android:id/content  [0,106][540,1212]\n", "a.b", false},
		{"no content seen", "[a.b]\n  text: Hi  [0,0][1,1]\n" + system, "a.b", false},
		{"no app", "[a.b]\n  id: android:id/content  [0,106][540,1212]\n", "", false},
		{"nothing", "", "a.b", false},
	} {
		if got := blank(c.screen, c.app); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}
