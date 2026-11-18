package utils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestToGMT7(t *testing.T) {
	utcTime := time.Date(2026, 11, 18, 2, 16, 0, 0, time.UTC)
	got := ToGMT7(utcTime)

	_, offset := got.Zone()
	assert.Equal(t, 7*60*60, offset)
	assert.Equal(t, "2026-11-18T09:16:00+07:00", got.Format(time.RFC3339))

	midnightRollover := time.Date(2026, 11, 18, 18, 30, 0, 0, time.UTC)
	assert.Equal(t, "2026-11-19T01:30:00+07:00", ToGMT7(midnightRollover).Format(time.RFC3339))

	assert.True(t, ToGMT7(time.Time{}).IsZero())
}

func TestResolvePostgresLocation(t *testing.T) {
	assert.Equal(t, time.UTC, ResolvePostgresLocation("UTC"))
	assert.Equal(t, time.UTC, ResolvePostgresLocation("Etc/UTC"))
	assert.Equal(t, time.UTC, ResolvePostgresLocation(""))
	assert.Equal(t, time.Local, ResolvePostgresLocation("localtime"))

	jakarta := ResolvePostgresLocation("Asia/Jakarta")
	_, jakartaOffset := time.Date(2026, 11, 18, 9, 0, 0, 0, jakarta).Zone()
	assert.Equal(t, 7*60*60, jakartaOffset)

	wib := ResolvePostgresLocation("WIB")
	_, wibOffset := time.Date(2026, 11, 18, 9, 0, 0, 0, wib).Zone()
	assert.Equal(t, 7*60*60, wibOffset)

	assert.Equal(t, time.UTC, ResolvePostgresLocation("Invalid/Zone"))
}
