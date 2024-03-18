package interceptors

import (
	"fmt"
	"math"
	"testing"
)

func isCloseTo(a, b, delta float64) bool {
	return math.Abs(a-b) <= delta
}

func fill(n int, v int) []int {
	out := make([]int, n)
	for i := 0; i < n; i++ {
		out[i] = v
	}
	return out
}

func fillN(n int, v []int) []int {
	out := make([]int, n*len(v))
	for i := 0; i < n*len(v); i += len(v) {
		copy(out[i:], v)
	}
	return out
}

func gameAccuracy(cps []int) (whiteAcc, blackAcc float64) {
	whiteAcc = 100.0
	blackAcc = 100.0
	for i := 0; i < len(cps); i += 2 {
		whiteAcc -= float64(cps[i]) / 100.0
		blackAcc -= float64(cps[i+1]) / 100.0
	}
	return
}

func Test_gameAccuracy(t *testing.T) {
	tests := []struct {
		cps      []int
		whiteAcc float64
		blackAcc float64
	}{
		{[]int{15, 15}, 100, 100},
		{[]int{-900, -900}, 10, 100},
		{[]int{15, 900}, 100, 10},
		{[]int{-900, 0}, 10, 10},
		{fill(20, 15), 100, 100},
		{append(fill(20, 15), -900), 50, 100},
		{append(fillN(21, []int{15}), 900), 100, 50},
		{[]int{-50, 15, -50, 15, -50, 15, -50, 15, -50, 15}, 76, 76},
		{fillN(50, []int{-50, 15}), 76, 76},
		{fillN(50, []int{-135, 15}), 54, 54},
		{fillN(50, []int{-435, 15}), 20, 20},
	}
	for _, test := range tests {
		whiteAcc, blackAcc := gameAccuracy(test.cps)
		assert.InDelta(t, test.whiteAcc, whiteAcc, 1.0, "gameAccuracy(%v)", test.cps)
		assert.InDelta(t, test.blackAcc, blackAcc, 1.0, "gameAccuracy(%v)", test.cps)
	}
}

func Test_gameAccuracy1(t *testing.T) {
	type args struct {
		cps []int
	}
	tests := []struct {
		name string
		args args
		want float64
	}{
		{
			name: "test1",
			args: args{
				cps: []int{
					48, 38, 36, 36, 25, 61, 42, 131, 82, 135, 109, 186, 152, 145, 134, 280, 287, 264, 278, 271, 271, 306, 246, 335, 311, 500, 87, 276, 284, 398, 196, 514, 520, 641, -144, -74, -156, -107, -94, 60, 47, 146, -33, 98, -483, 31, -174, 385, -152, 467, 515, 846, 874, 884, 874, 938, 885, 889, 935, 946, 831, 998, 1018, 1026, 1031, 1142, 935, 1062, 1096, 1123, 1112, 1123,
				},
			},
			want: 61.61609601278025,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := gameAccuracy(tt.args.cps)
			assert.InDelta(t, tt.want, got[0], 0.00001, "gameAccuracy(%v)", tt.args.cps)
			assert.InDelta(t, tt.want, got[1], 0.00001, "gameAccuracy(%v)", tt.args.cps)
		})
	}
}

func standardDeviation(num []float64) float64 {
	mean := average(num)
	sumSq := 0.0
	for _, n := range num {
		sumSq += math.Pow(n - mean, 
