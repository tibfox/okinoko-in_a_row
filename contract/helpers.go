package main

import (
	"encoding/json"
	"fmt"
	"okinoko-in_a_row/sdk"
	"strconv"
	"strings"
)

// ---------- JSON Conversions ----------

// ToJSON marshals any Go value `v` into a JSON string.
// The `objectType` string is used in error messages to indicate context.
// Aborts execution if JSON marshalling fails.
func ToJSON[T any](v T, objectType string) string {
	b, err := json.Marshal(v)
	if err != nil {
		sdk.Abort("failed to marshal " + objectType)
	}
	return string(b)
}

// ---------- String/Number Helpers ----------

// StringToUInt64 converts a string pointer into a uint64 integer in base 10.
// Aborts execution if the pointer is nil or if parsing fails.
func StringToUInt64(ptr *string) uint64 {
	if ptr == nil {
		sdk.Abort("input is empty")
	}
	val, err := strconv.ParseUint(*ptr, 10, 64)
	if err != nil {
		sdk.Abort(fmt.Sprintf("failed to parse '%s' to uint64: %w", *ptr, err))
	}
	return val
}

// UInt64ToString converts an unsigned 64-bit integer to its decimal string form.
//
//go:inline
func UInt64ToString(val uint64) string {
	return strconv.FormatUint(val, 10)
}

// ---------- Transfer Intent Helpers ----------

// TransferAllow represents a parsed "transfer.allow" intent,
// containing an allowed token type and maximum transfer limit.
// Limit is expressed in floating-point token units (e.g., 1.234 HIVE).
type TransferAllow struct {
	Limit float64
	Token sdk.Asset
}

// validAssets enumerates tokens accepted in betting/transfers.
var validAssets = []string{sdk.AssetHbd.String(), sdk.AssetHive.String()}

// isValidAsset reports whether the given token name matches any supported asset.
//
//go:inline
func isValidAsset(token string) bool {
	for _, a := range validAssets {
		if token == a {
			return true
		}
	}
	return false
}

// GetFirstTransferAllow scans a list of intents to find the
// first valid "transfer.allow" intent that specifies a supported token.
// Returns nil if no matching intent is found.
// Aborts if a transfer.allow intent contains an unsupported token or invalid limit.
func GetFirstTransferAllow(intents []sdk.Intent) *TransferAllow {
	for _, intent := range intents {
		if intent.Type == "transfer.allow" {
			token := intent.Args["token"]
			if !isValidAsset(token) {
				sdk.Abort("invalid intent token")
			}
			limitStr := intent.Args["limit"]
			limit, err := strconv.ParseFloat(limitStr, 64)
			if err != nil {
				sdk.Abort("invalid intent limit")
			}
			return &TransferAllow{
				Limit: limit,
				Token: sdk.Asset(token),
			}
		}
	}
	return nil
}

// boardToASCII converts the game's compressed binary board (2 bits per cell)
// into an ASCII string of digits:
//
//	'0' = empty, '1' = creator/X, '2' = opponent/O
//
// The output is row-major, with no separators.
func boardToASCII(g *Game) string {
	rows, cols := dims(g.Type)
	total := rows * cols
	out := make([]byte, total)
	for i := 0; i < total; i++ {
		byteIdx := i / 4
		bitShift := (i % 4) * 2
		cell := (g.Board[byteIdx] >> bitShift) & 0x03
		out[i] = '0' + cell
	}
	return string(out)
}

// ---------- Date/Time Conversions ----------

// unixToISO8601 converts a UNIX timestamp (seconds since epoch) into a fixed-format
// "YYYY-MM-DDThh:mm:ss" string in UTC. The format does not include timezone suffix.
// This function performs arithmetic using the proleptic Gregorian calendar.
func unixToISO8601(ts uint64) string {
	// Split into whole days + remainder seconds
	days := int64(ts / 86400)
	sec := int64(ts % 86400)
	hour := sec / 3600
	sec %= 3600
	minute := sec / 60
	second := sec % 60

	// Howard Hinnant's civil_from_days algorithm
	z := days + 719468
	var era int64
	if z >= 0 {
		era = z / 146097
	} else {
		era = (z - 146096) / 146097
	}
	doe := z - era*146097
	yoe := (doe - doe/1460 + doe/36524 - doe/146096) / 365
	y := yoe + era*400
	doy := doe - (365*yoe + yoe/4 - yoe/100 + yoe/400)
	mp := (5*doy + 2) / 153
	d := doy - (153*mp+2)/5 + 1
	m := mp + 3 - 12*((mp+3)/13)
	y += (mp + 3) / 13

	year := int(y)
	month := int(m)
	day := int(d)

	// Pre-allocate fixed 19-byte buffer
	var buf [19]byte
	buf[0] = '0' + byte((year/1000)%10)
	buf[1] = '0' + byte((year/100)%10)
	buf[2] = '0' + byte((year/10)%10)
	buf[3] = '0' + byte(year%10)
	buf[4] = '-'
	buf[5] = '0' + byte((month/10)%10)
	buf[6] = '0' + byte(month%10)
	buf[7] = '-'
	buf[8] = '0' + byte((day/10)%10)
	buf[9] = '0' + byte(day%10)
	buf[10] = 'T'
	buf[11] = '0' + byte((hour/10)%10)
	buf[12] = '0' + byte(hour%10)
	buf[13] = ':'
	buf[14] = '0' + byte((minute/10)%10)
	buf[15] = '0' + byte(minute%10)
	buf[16] = ':'
	buf[17] = '0' + byte((second/10)%10)
	buf[18] = '0' + byte(second%10)

	return string(buf[:])
}

// parseISO8601ToUnix parses strings in the "YYYY-MM-DDThh:mm:ss" format
// and converts them to UNIX seconds. Assumes valid ASCII digits.
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

// ---------- Fast Human-ABI Parsing Helpers ----------

// nextField splits the input string on the first '|' character,
// returning the field before it, and mutating the string pointer to
// point to the remaining content after the separator.
// If no '|' exists, returns the whole string and empties the input.
//
// This function is used to parse lightweight ABI strings without allocation.
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

// parseU64Fast parses an ASCII decimal string into an unsigned 64-bit integer.
// It assumes valid input and does not perform error checking.
//
//go:inline
func parseU64Fast(s string) uint64 {
	var n uint64
	for i := 0; i < len(s); i++ {
		n = n*10 + uint64(s[i]-'0')
	}
	return n
}

// parseU8Fast is a fast parser for uint8 from ASCII decimal string.
// Assumes valid digits.
//
//go:inline
func parseU8Fast(s string) uint8 {
	var n uint8
	for i := 0; i < len(s); i++ {
		n = n*10 + uint8(s[i]-'0')
	}
	return n
}

// appendU64 appends the ASCII decimal representation of v to dst.
// Used for ABI construction without extra allocations.
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

// appendU16 appends v as decimal ASCII to dst.
//
//go:inline
func appendU16(dst []byte, v uint16) []byte { return appendU64(dst, uint64(v)) }

// appendU8 appends v as decimal ASCII to dst.
//
//go:inline
func appendU8(dst []byte, v uint8) []byte { return appendU64(dst, uint64(v)) }

// strToUint16Fast converts an ASCII decimal string to uint16.
// Assumes valid digits and no errors.
//
//go:inline
func strToUint16Fast(s string) uint16 {
	var n uint16
	for i := 0; i < len(s); i++ {
		n = n*10 + uint16(s[i]-'0')
	}
	return n
}

// strToUint8Fast converts an ASCII decimal string to uint8.
//
//go:inline
func strToUint8Fast(s string) uint8 {
	var n uint8
	for i := 0; i < len(s); i++ {
		n = n*10 + uint8(s[i]-'0')
	}
	return n
}

// isLeapYear returns true if year is a leap year per Gregorian rules.
//
//go:inline
func isLeapYear(year uint16) bool {
	y := int(year)
	return (y%4 == 0 && y%100 != 0) || (y%400 == 0)
}

// daysSinceUnixEpoch computes the number of whole days between 1970-01-01
// and the provided date. Used for UNIX timestamp calculation.
func daysSinceUnixEpoch(year uint16, month uint8, day uint8) uint64 {
	y := int(year) - 1970
	days := uint64(y * 365)

	// Add leap days
	days += uint64((y+2)/4 - (y+70)/100 + (y+370)/400)

	// Month lengths
	var monthDays = [12]uint8{31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}
	for i := uint8(1); i < month; i++ {
		days += uint64(monthDays[i-1])
		if i == 2 && isLeapYear(year) {
			days++
		}
	}

	return days + uint64(day-1)
}

// require aborts execution with message msg if condition cond is false.
// Used throughout the contract for invariant validation.
func require(cond bool, msg string) {
	if !cond {
		sdk.Abort(msg)
	}
}
