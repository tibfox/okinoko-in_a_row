package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"vsc_tictactoe/sdk"
)

// ---------- JSON Conversions ----------

// ToJSON marshals a Go value into a JSON string.
// Aborts execution if marshalling fails.
func ToJSON[T any](v T, objectType string) string {
	b, err := json.Marshal(v)
	if err != nil {
		sdk.Abort("failed to marshal " + objectType)
	}
	return string(b)
}

// ---------- String/Number Helpers ----------

// StringToUInt64 converts a string pointer into a uint64.
// Aborts if the pointer is nil or parsing fails.
func StringToUInt64(ptr *string) uint64 {
	if ptr == nil {
		sdk.Abort("input is empty")
	}
	val, err := strconv.ParseUint(*ptr, 10, 64) // base 10, 64-bit
	if err != nil {
		sdk.Abort(fmt.Sprintf("failed to parse '%s' to uint64: %w", *ptr, err))
	}
	return val
}

// UInt64ToString converts a uint64 to its decimal string representation.
func UInt64ToString(val uint64) string {
	return strconv.FormatUint(val, 10)
}

// ---------- Transfer Intent Helpers ----------

// TransferAllow represents a parsed transfer.allow intent,
// including the limit (amount) and token (asset).
type TransferAllow struct {
	Limit float64
	Token sdk.Asset
}

// validAssets defines the list of supported assets for transfer intents.
var validAssets = []string{sdk.AssetHbd.String(), sdk.AssetHive.String()}

// isValidAsset checks whether a given token string is a supported asset.
func isValidAsset(token string) bool {
	for _, a := range validAssets {
		if token == a {
			return true
		}
	}
	return false
}

// GetFirstTransferAllow searches the provided intents and returns the first
// valid transfer.allow intent as a TransferAllow. Returns nil if none exist.
func GetFirstTransferAllow(intents []sdk.Intent) *TransferAllow {
	for _, intent := range intents {
		if intent.Type == "transfer.allow" {
			token := intent.Args["token"]
			// If we have a transfer.allow intent but the asset is not valid, abort.
			if !isValidAsset(token) {
				sdk.Abort("invalid intent token")
			}
			limitStr := intent.Args["limit"]
			limit, err := strconv.ParseFloat(limitStr, 64)
			if err != nil {
				sdk.Abort("invalid intent limit")
			}
			ta := &TransferAllow{
				Limit: limit,
				Token: sdk.Asset(token),
			}
			return ta
		}
	}
	return nil
}

// ---------- Game Counter Helpers ----------

// getGameCount retrieves the current game counter from state.
// Returns 0 if no counter exists.
func getGameCount() uint64 {
	ptr := sdk.StateGetObject("g:count")
	if ptr == nil || *ptr == "" {
		return 0
	}
	return StringToUInt64(ptr)
}

// setGameCount updates the game counter in state to the given value.
func setGameCount(newCount uint64) {
	sdk.StateSetObject("g:count", UInt64ToString(newCount))
}

// Converts compressed board (2 bits per cell) into ASCII string:
// '0' = unset, '1' = creator (X), '2' = joiner (O)
func boardToASCII(g *Game) string {
	rows, cols := dims(g.Type)
	total := rows * cols
	out := make([]byte, total)
	for i := 0; i < total; i++ {
		byteIdx := i / 4
		bitShift := (i % 4) * 2
		cell := (g.Board[byteIdx] >> bitShift) & 0x03
		out[i] = '0' + cell // converts 0 → '0', 1 → '1', 2 → '2'
	}
	return string(out)
}

// date conversions

// Converts UNIX timestamp (seconds) to "YYYY-MM-DDThh:mm:ss" (UTC, no 'Z')
// Pure integer math; no allocations beyond the fixed 19-byte buffer.
func unixToISO8601(ts uint64) string {
	// Split into day + time (use int64 consistently)
	days := int64(ts / 86400)
	sec := int64(ts % 86400)
	hour := sec / 3600
	sec %= 3600
	minute := sec / 60
	second := sec % 60

	// Howard Hinnant's civil_from_days (reversible, leap-safe)
	z := days + 719468
	var era int64
	if z >= 0 {
		era = z / 146097
	} else {
		era = (z - 146096) / 146097
	}
	doe := z - era*146097                                  // [0, 146096]
	yoe := (doe - doe/1460 + doe/36524 - doe/146096) / 365 // [0, 399]
	y := yoe + era*400
	doy := doe - (365*yoe + yoe/4 - yoe/100 + yoe/400) // [0, 365]
	mp := (5*doy + 2) / 153                            // [0, 11]
	d := doy - (153*mp+2)/5 + 1                        // [1, 31]
	m := mp + 3 - 12*((mp+3)/13)                       // [1, 12]
	y += (mp + 3) / 13

	year := int(y)
	month := int(m)
	day := int(d)

	// Format "YYYY-MM-DDThh:mm:ss" into a fixed-size buffer
	var buf [19]byte

	// Year (4 digits)
	buf[0] = '0' + byte((year/1000)%10)
	buf[1] = '0' + byte((year/100)%10)
	buf[2] = '0' + byte((year/10)%10)
	buf[3] = '0' + byte(year%10)

	buf[4] = '-'

	// Month (2 digits)
	buf[5] = '0' + byte((month/10)%10)
	buf[6] = '0' + byte(month%10)

	buf[7] = '-'

	// Day (2 digits)
	buf[8] = '0' + byte((day/10)%10)
	buf[9] = '0' + byte(day%10)

	buf[10] = 'T'

	// Hour (2 digits)
	h := int(hour)
	buf[11] = '0' + byte((h/10)%10)
	buf[12] = '0' + byte(h%10)

	buf[13] = ':'

	// Minute (2 digits)
	min := int(minute)
	buf[14] = '0' + byte((min/10)%10)
	buf[15] = '0' + byte(min%10)

	buf[16] = ':'

	// Second (2 digits)
	sec2 := int(second)
	buf[17] = '0' + byte((sec2/10)%10)
	buf[18] = '0' + byte(sec2%10)

	return string(buf[:])
}

// Parse "YYYY-MM-DDThh:mm:ss" into UNIX seconds
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

// ---------- Fast human-ABI helpers ----------

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

// decimal -> uint with no error path; assume valid ASCII digits
func parseU64Fast(s string) uint64 {
	var n uint64
	for i := 0; i < len(s); i++ {
		n = n*10 + uint64(s[i]-'0')
	}
	return n
}

func parseU8Fast(s string) uint8 {
	var n uint8
	for i := 0; i < len(s); i++ {
		n = n*10 + uint8(s[i]-'0')
	}
	return n
}

// decimal formatting (uint -> ascii) with no allocations beyond dst growth
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

// ----- conversion helpers --------
// Convert string of digits to uint16 (no errors assumed)
func strToUint16Fast(s string) uint16 {
	var n uint16
	for i := 0; i < len(s); i++ {
		n = n*10 + uint16(s[i]-'0')
	}
	return n
}

// Convert string of digits to uint8 (no errors assumed)
func strToUint8Fast(s string) uint8 {
	var n uint8
	for i := 0; i < len(s); i++ {
		n = n*10 + uint8(s[i]-'0')
	}
	return n
}

// Checks if a given year is a leap year
func isLeapYear(year uint16) bool {
	y := int(year)
	return (y%4 == 0 && y%100 != 0) || (y%400 == 0)
}

// Days from 1970-01-01 to the given date (UTC)
func daysSinceUnixEpoch(year uint16, month uint8, day uint8) uint64 {
	// Years since epoch
	y := int(year) - 1970
	// Add days for all prior years
	days := uint64(y * 365)

	// Add leap days
	// Equivalent to: floor((year-1969)/4) - floor((year-1901)/100) + floor((year-1601)/400)
	days += uint64((y+2)/4 - (y+70)/100 + (y+370)/400)

	// Month lengths
	var monthDays = [12]uint8{31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}
	for i := uint8(1); i < month; i++ {
		days += uint64(monthDays[i-1])
		if i == 2 && isLeapYear(year) { // Add leap day after February
			days++
		}
	}

	// Add days in current month (subtract 1 because the epoch day is day 1)
	return days + uint64(day-1)
}

func require(cond bool, msg string) {
	if !cond {
		sdk.Abort(msg)
	}
}
