package auth

import "golang.org/x/crypto/bcrypt"

const minPasswordLength = 8

var dummyPasswordHash string

func init() {
	hash, err := bcrypt.GenerateFromPassword([]byte("timing-equalization"), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	dummyPasswordHash = string(hash)
}

// HashPassword returns a bcrypt hash. The password is never stored in plaintext.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func checkPassword(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

func validPassword(password string) bool {
	return len(password) >= minPasswordLength && len(password) <= 72
}
