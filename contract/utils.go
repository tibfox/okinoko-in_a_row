package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"okinoko-in_a_row/sdk"
	"strconv"
	"strings"
)

//
// ---------- JSON Conversions ----------
//

// ToJSON marshals a value into JSON text for off-chain use.
// Panics via sdk.Abort on failure rather than returning an error
// since contracts can't bubble error chains well.
func ToJSON[T any](v T, objectType string) string {
	b, err := json.Marshal(v)
	if err != nil {
		sdk.Abort("failed to marshal " + objectType)
	}
	return string(b)
}

//
// ---------- UInt/String Helpers ----------
//

// StringToUInt64 parses a decimal string pointer into a uint64.
// Aborts if input is nil or not a valid integer.
func StringToUInt64(ptr *string) uint64 {
	if ptr == nil {
		sdk.Abort("input is empty")
	}
	val, err := strconv.ParseUint(*ptr, 10, 64)
	if err != nil {
		sdk.Abort(fmt.Sprintf("failed to parse '%s' to uint64: %v", *ptr, err))
	}
	return val
}

// UInt64ToString returns the decimal text form of a uint64.
func UInt64ToString(val uint64) string {
	return strconv.FormatUint(val, 10)
}

//
// ---------- Time Helpers ----------
//

// parseISO8601ToUnix decodes a timestamp of the form YYYY-MM-DDThh:mm:ss (UTC)
// into unix seconds. It trusts ASCII digits and avoids allocations/stdlib time.
func parseISO8601ToUnix(s string) uint64 {
	year := strToUint16Fast(s[0:4])
	month := strToUint8Fast(s[5:7])
	day := strToUint8Fast(s[8:10])
	hour := strToUint8Fast(s[11:13])
	minute := strToUint8Fast(s[14:16])
	second := strToUint8Fast(s[17:19])

	days := daysSinceUnixEpoch(year, month, day)
	return days*86400 + uint64(hour)*3600 + uint64(minute)*60 + uint64(second)
}

// Fast fixed-width numeric scans (no bounds checks here by design).
func strToUint16Fast(s string) uint16 {
	var n uint16
	for i := 0; i < len(s); i++ {
		n = n*10 + uint16(s[i]-'0')
	}
	return n
}
func strToUint8Fast(s string) uint8 {
	var n uint8
	for i := 0; i < len(s); i++ {
		n = n*10 + uint8(s[i]-'0')
	}
	return n
}

// isLeapYear uses the Gregorian rules (same as Unix).
func isLeapYear(year uint16) bool {
	y := int(year)
	return (y%4 == 0 && y%100 != 0) || (y%400 == 0)
}

// daysSinceUnixEpoch counts days from 1970-01-01 up to the given date.
// Used to avoid importing time pkg in WASM builds.
func daysSinceUnixEpoch(year uint16, month uint8, day uint8) uint64 {
	y := int(year) - 1970
	days := uint64(y * 365)
	days += uint64((y+2)/4 - (y+70)/100 + (y+370)/400)

	var monthDays = [12]uint8{31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}
	for i := uint8(1); i < month; i++ {
		days += uint64(monthDays[i-1])
		if i == 2 && isLeapYear(year) {
			days++
		}
	}
	return days + uint64(day-1)
}

//
// ---------- Parsing Helpers ----------
//

// nextField splits a string at the first '|' and advances the pointer,
// returning the left field. Used by the lightweight wire protocol.
func nextField(s *string) string {
	i := strings.IndexByte(*s, '|')
	if i < 0 {
		f := *s
		*s = ""
		return f
	}
	f := (*s)[:i]
	*s = (*s)[i+1:]
	return f
}

// parseU64Fast parses only ASCII digits to uint64 (no spaces, no signs).
func parseU64Fast(s string) uint64 {
	var n uint64
	for i := 0; i < len(s); i++ {
		n = n*10 + uint64(s[i]-'0')
	}
	return n
}

// parseU8Fast extracts a small uint (0-255) from decimal ASCII.
func parseU8Fast(s string) uint8 {
	var n uint8
	for i := 0; i < len(s); i++ {
		n = n*10 + uint8(s[i]-'0')
	}
	return n
}

// appendU64 prints a uint64 in decimal into an existing buffer.
// Used to build compact responses without fmt overhead.
func appendU64(dst []byte, v uint64) []byte {
	if v == 0 {
		return append(dst, '0')
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return append(dst, buf[i:]...)
}

func appendU16(dst []byte, v uint16) []byte { return appendU64(dst, uint64(v)) }
func appendU8(dst []byte, v uint8) []byte   { return appendU64(dst, uint64(v)) }

//
// ---------- Require ----------
//

// require aborts execution if cond is false.
// Kept tiny because it's called a lot in contract flows.
func require(cond bool, msg string) {
	if !cond {
		sdk.Abort(msg)
	}
}

//
// ---------- Binary Reader ----------
//

// rd walks a byte slice and provides small decode helpers.
// It panics via sdk.Abort on overflow to avoid silent corruption.
type rd struct {
	b []byte // raw buffer
	i int    // read cursor
}

func (r *rd) need(n int) {
	if r.i+n > len(r.b) {
		sdk.Abort("decode overflow")
	}
}

// u8 reads one byte.
func (r *rd) u8() byte {
	r.need(1)
	v := r.b[r.i]
	r.i++
	return v
}

// u16 reads a big-endian uint16.
func (r *rd) u16() uint16 {
	r.need(2)
	v := binary.BigEndian.Uint16(r.b[r.i : r.i+2])
	r.i += 2
	return v
}

// str reads a length-prefixed UTF-8 string (2-byte length).
func (r *rd) str() string {
	l := int(r.u16())
	r.need(l)
	v := string(r.b[r.i : r.i+l])
	r.i += l
	return v
}

// u64 reads a big-endian uint64.
func (r *rd) u64() uint64 {
	r.need(8)
	v := binary.BigEndian.Uint64(r.b[r.i : r.i+8])
	r.i += 8
	return v
}

// bytes returns the next n bytes verbatim.
func (r *rd) bytes(n int) []byte {
	r.need(n)
	v := r.b[r.i : r.i+n]
	r.i += n
	return v
}

// appendString16 writes a 2-byte length then raw string bytes.
func appendString16(out []byte, s string) []byte {
	if len(s) > 65535 {
		sdk.Abort("string too long")
	}
	var tmp [2]byte
	binary.BigEndian.PutUint16(tmp[:], uint16(len(s)))
	out = append(out, tmp[:]...)
	return append(out, s...)
}

//
// ---------- Fixed-Point Parsing ----------
//

// parseFixedPoint3 parses a decimal with up to 3 fractional digits
// and returns value scaled by 1000. No floats, no allocations.
// Examples:
//
//	"1"     -> 1000
//	"1.2"   -> 1200
//	"1.23"  -> 1230
//	"1.234" -> 1234
func parseFixedPoint3(s string) uint64 {
	n := len(s)
	if n == 0 {
		return 0
	}

	var intPart uint64
	var fracPart uint64
	var fracDigits int
	dotSeen := false

	for i := 0; i < n; i++ {
		c := s[i]

		if c == '.' {
			require(!dotSeen, "invalid number: multiple dots")
			dotSeen = true
			continue
		}

		require(c >= '0' && c <= '9', "invalid character in number")
		d := uint64(c - '0')

		if !dotSeen {
			intPart = (intPart << 3) + (intPart << 1) + d // mul 10 without *
		} else {
			require(fracDigits < 3, "too many fractional digits")
			fracDigits++
			fracPart = (fracPart << 3) + (fracPart << 1) + d
		}
	}

	// scale fractional portion to 3 digits
	switch fracDigits {
	case 0:
		fracPart = 0
	case 1:
		fracPart *= 100
	case 2:
		fracPart *= 10
	}
	return intPart*1000 + fracPart
}
