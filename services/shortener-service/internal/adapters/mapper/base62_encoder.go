package mapper

const base62Chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// EncodeToBase62 converts a number to base62 string
func EncodeToBase62(num int64) string {
	if num == 0 {
		return string(base62Chars[0])
	}

	base := int64(len(base62Chars))
	encoded := ""

	for num > 0 {
		remainder := num % base
		encoded = string(base62Chars[remainder]) + encoded
		num = num / base
	}

	return encoded
}

// DecodeFromBase62 converts base62 string back to number
func DecodeFromBase62(str string) int64 {
	base := int64(len(base62Chars))
	decoded := int64(0)

	for i := 0; i < len(str); i++ {
		char := str[i]
		var value int64

		// Find position of character in base62Chars
		for j := 0; j < len(base62Chars); j++ {
			if base62Chars[j] == char {
				value = int64(j)
				break
			}
		}

		decoded = decoded*base + value
	}

	return decoded
}
