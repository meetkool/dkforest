package utils

import (
	"strconv"
	"testing"
)

// ParseInt64OrDefault parses the input string as an int64 and returns the result or the default value if the parsing fails.
// It also returns an error if the parsing fails.
func ParseInt64OrDefault(v string, d int64) (int64, error) {
	i, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return d, err
	}
	return i, nil
}

func TestParseInt64OrDefault(t *testing.T) {
	type args struct {
		v string
		d int64
	}
	type want struct {
		out int6
