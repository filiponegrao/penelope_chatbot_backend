package tools

import "regexp"

// nome exato: ValidateEmail (como seu controller chama)
func ValidateEmail(email string) bool {
	re := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	return re.MatchString(email)
}

// nome exato: CheckPassword (como o models/user.go do Venditto chama)
func CheckPassword(password string) string {
	if len(password) < 6 {
		return "password"
	}
	return ""
}
