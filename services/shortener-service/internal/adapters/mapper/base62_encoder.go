package mapper

import "fmt"

const base62Chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

var base62Lookup [256]int8

func init() {
	for i := range base62Lookup {
		base62Lookup[i] = -1
	}
	for i := 0; i < len(base62Chars); i++ {
		base62Lookup[base62Chars[i]] = int8(i)
	}
}

func EncodeToBase62(num int64) string {
	if num <= 0 {
		return string(base62Chars[0])
	}

	const base = int64(62)
	buf := make([]byte, 0, 11)
	for num > 0 {
		buf = append(buf, base62Chars[num%base])
		num /= base
	}

	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return string(buf)
}

func DecodeFromBase62(s string) (int64, error) {
	if s == "" {
		return 0, fmt.Errorf("empty base62 string")
	}

	const base = int64(62)
	var n int64
	for i := 0; i < len(s); i++ {
		v := base62Lookup[s[i]]
		if v < 0 {
			return 0, fmt.Errorf("invalid base62 character %q at position %d", s[i], i)
		}
		n = n*base + int64(v)
	}
	return n, nil
}
