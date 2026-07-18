package shared

import "encoding/json"

// Color is a palette color value that accepts both JSON strings (hex like
// "#3ddc84") and JSON numbers (ANSI-256 like 212). The string form is used
// by lipgloss.Color(); numbers are stringified so "212" works the same as
// 212.
type Color string

// UnmarshalJSON accepts both string and number JSON tokens.
func (c *Color) UnmarshalJSON(data []byte) error {
	// Try string first.
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*c = Color(s)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(data, &n); err != nil {
		return err
	}
	*c = Color(string(n))
	return nil
}

// String returns the color as a string suitable for lipgloss.Color().
func (c Color) String() string {
	return string(c)
}
