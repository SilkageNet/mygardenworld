package apiserver

import "testing"

func TestValidatePasswordRejectsWeakPasswords(t *testing.T) {
	for _, password := range []string{
		"short",
		"change-me-first",
		"aaaaaaaaaaaa",
		"Password1234",
	} {
		t.Run(password, func(t *testing.T) {
			if err := ValidatePassword(password); err == nil {
				t.Fatal("ValidatePassword succeeded, want error")
			}
		})
	}
}

func TestValidatePasswordAcceptsStrongPassword(t *testing.T) {
	if err := ValidatePassword("LongerPass123!"); err != nil {
		t.Fatalf("ValidatePassword returned error: %v", err)
	}
}
