package mock

import (
	"encoding/json"
	"testing"
)

func TestRedactionKeepsForm(t *testing.T) {
	r := &real{settings: Settings{RedactValues: []string{"user_name=kit"}}}
	in := `{"email":"a@b.co","email_verified_at":"2024-05-06T07:08:09.123+02:00","birthdate":"1990-03-04",
"phone":"+34 600 111 222","first_name":"Ana","last_name":"Ruiz","full_name":"Ana Ruiz","username":"ana9",
"zip":"28001","password":"hunter2","age":41,"mail_ok":true}`
	var v any
	if err := json.Unmarshal([]byte(in), &v); err != nil {
		t.Fatal(err)
	}
	got := r.cleanValue(v, false).(map[string]any)
	want := map[string]any{"email": nobody, "email_verified_at": "2000-01-01T00:00:00.000+00:00", "birthdate": "2000-01-01",
		"phone": "+12025550123", "first_name": "Alex", "last_name": "Doe", "full_name": "Alex Doe", "username": "kit",
		"zip": "00000", "password": redacted, "mail_ok": true}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("%s: got %v, want %v", k, got[k], w)
		}
	}
}
