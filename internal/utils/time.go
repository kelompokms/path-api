package utils

import (
	"strings"
	"time"
	_ "time/tzdata"
)

const gmt7OffsetSeconds = 7 * 60 * 60

var gmt7Location = loadGMT7Location()

func loadGMT7Location() *time.Location {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		return time.FixedZone("GMT+7", gmt7OffsetSeconds)
	}
	return loc
}

// GMT7Location returns the Asia/Jakarta (GMT+7 / WIB) time.Location.
func GMT7Location() *time.Location {
	return gmt7Location
}

// NowGMT7 returns the current time in GMT+7 (Asia/Jakarta).
func NowGMT7() time.Time {
	return time.Now().In(gmt7Location)
}

// ToGMT7 converts t into GMT+7 (Asia/Jakarta) while preserving zero values.
func ToGMT7(t time.Time) time.Time {
	if t.IsZero() {
		return t
	}
	return t.In(gmt7Location)
}

// ResolvePostgresLocation maps a PostgreSQL session TimeZone name to a Go
// *time.Location so pgx scans TIMESTAMP WITHOUT TIME ZONE values in the same
// zone PostgreSQL used to write them.
func ResolvePostgresLocation(tzName string) *time.Location {
	trimmed := strings.TrimSpace(tzName)
	if trimmed == "" {
		return time.UTC
	}

	switch strings.ToUpper(trimmed) {
	case "UTC", "ETC/UTC", "GMT", "ETC/GMT", "UNIVERSAL":
		return time.UTC
	case "LOCALTIME":
		return time.Local
	case "WIB":
		return gmt7Location
	}

	loc, err := time.LoadLocation(trimmed)
	if err != nil {
		return time.UTC
	}
	return loc
}
