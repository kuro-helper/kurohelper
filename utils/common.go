package utils

import (
	"os"
	"strconv"
	"time"

	kurohelpererrors "kurohelper/errors"
)

// 取得env(轉型int版)
func GetEnvInt(key string, def int) int {
	if val := os.Getenv(key); val != "" {
		if v, err := strconv.Atoi(val); err == nil {
			return v
		}
	}
	return def
}

func ParseYYYYMMDD(s string) (time.Time, error) {
	if len(s) != 8 {
		return time.Time{}, kurohelpererrors.ErrTimeWrongFormat
	}

	if _, err := strconv.Atoi(s); err != nil {
		return time.Time{}, kurohelpererrors.ErrTimeWrongFormat
	}

	t, err := time.Parse("20060102", s)
	if err != nil {
		return time.Time{}, kurohelpererrors.ErrTimeWrongFormat
	}

	return t, nil
}

func IsAllHanziOrDigit(s string) bool {
	if len(s) == 0 {
		return false
	}

	for _, r := range s {
		if isDigit(r) {
			continue
		}
		if isHanzi(r) {
			continue
		}
		return false
	}

	return true
}

func isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

// 是否為中文字
func isHanzi(r rune) bool {
	switch {
	// CJK Unified Ideographs (4E00–9FFF)
	case r >= 0x4E00 && r <= 0x9FFF:
		return true
	// CJK Unified Ideographs Extension A (3400–4DBF)
	case r >= 0x3400 && r <= 0x4DBF:
		return true
	// CJK Unified Ideographs Extension B (20000–2A6DF)
	case r >= 0x20000 && r <= 0x2A6DF:
		return true
	// CJK Unified Ideographs Extension C (2A700–2B73F)
	case r >= 0x2A700 && r <= 0x2B73F:
		return true
	// CJK Unified Ideographs Extension D (2B740–2B81F)
	case r >= 0x2B740 && r <= 0x2B81F:
		return true
	// CJK Unified Ideographs Extension E (2B820–2CEAF)
	case r >= 0x2B820 && r <= 0x2CEAF:
		return true
	// CJK Unified Ideographs Extension F (2CEB0–2EBEF)
	case r >= 0x2CEB0 && r <= 0x2EBEF:
		return true
	}
	return false
}
