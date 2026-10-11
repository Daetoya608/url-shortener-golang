// Package shortcode генерирует короткие коды и проверяет пользовательские alias.
package shortcode

import (
	"crypto/rand"
	"regexp"
)

// Length — длина генерируемого кода.
const Length = 7

// Alphabet — символы base62.
const Alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

var aliasRe = regexp.MustCompile(`^[A-Za-z0-9_-]{3,32}$`)

// Generate возвращает случайный код из Length символов base62 (crypto/rand).
func Generate() string {
	// Отбрасываем байты >= 248 (4*62), чтобы распределение было равномерным.
	const limit = 256 - 256%len(Alphabet)
	out := make([]byte, 0, Length)
	buf := make([]byte, Length*2)
	for len(out) < Length {
		// crypto/rand.Read не возвращает ошибок начиная с Go 1.24.
		_, _ = rand.Read(buf)
		for _, b := range buf {
			if int(b) >= limit {
				continue
			}
			out = append(out, Alphabet[int(b)%len(Alphabet)])
			if len(out) == Length {
				break
			}
		}
	}
	return string(out)
}

// ValidAlias сообщает, подходит ли s под формат alias: ^[A-Za-z0-9_-]{3,32}$.
func ValidAlias(s string) bool {
	return aliasRe.MatchString(s)
}
