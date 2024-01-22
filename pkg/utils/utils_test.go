package utils

import "testing"

func TestParseInt64OrDefault(t *testing.T) {
	type args struct {
		v string
		d int64
	}
	tests := []struct {
		name    string
		args    args
		wantOut int64
	}{
		{name: "", args: args{v: "", d: 10}, wantOut: 10},
		{name: "", args: args{v: "0", d: 10}, wantOut: 0},
		{name: "", args: args{v: "-1", d: 10}, wantOut: -1},
		{name: "", args: args{v: "1", d: 10}, wantOut: 1},
		{name: "", args: args{v: "a", d: 10}, wantOut: 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotOut := ParseInt64OrDefault(tt.args.v, tt.args.d); gotOut != tt.wantOut {
				t.Errorf("DoParseInt64OrDefault() = %v, want %v", gotOut, tt.wantOut)
			}
		})
	}
}
