package interceptors

import (
	"github.com/stretchr/testify/assert"
	"math"
	"testing"
)

func isCloseTo(a, b, delta float64) bool {
	return math.Abs(a-b) <= delta
}

func fill(n, v int) []int {
	out := make([]int, n)
	for i := 0; i < n; i++ {
		out[i] = v
	}
	return out
}

func fill1(n int, v []int) []int {
	out := make([]int, n*len(v))
	for i := 0; i < n*len(v); i += len(v) {
		for j := range v {
			out[i+j] = v[j]
		}
	}
	return out
}

func Test_gameAccuracy(t *testing.T) {
	// two good moves
	w, b := gameAccuracy([]int{15, 15})
	assert.True(t, isCloseTo(w, 100, 1))
	assert.True(t, isCloseTo(b, 100, 1))
	// white blunders on first move
	w, b = gameAccuracy([]int{-900, -900})
	assert.True(t, isCloseTo(w, 10, 5))
	assert.True(t, isCloseTo(b, 100, 1))
	// black blunders on first move
	w, b = gameAccuracy([]int{15, 900})
	assert.True(t, isCloseTo(w, 100, 1))
	assert.True(t, isCloseTo(b, 10, 5))
	// both blunder on first move
	w, b = gameAccuracy([]int{-900, 0})
	assert.True(t, isCloseTo(w, 10, 5))
	assert.True(t, isCloseTo(b, 10, 5))
	// 20 perfect moves
	w, b = gameAccuracy(fill(20, 15))
	assert.True(t, isCloseTo(w, 100, 1))
	assert.True(t, isCloseTo(b, 100, 1))
	// 20 perfect moves and a white blunder
	cps := fill(20, 15)
	cps = append(cps, -900)
	w, b = gameAccuracy(cps)
	assert.True(t, isCloseTo(w, 50, 5))
	assert.True(t, isCloseTo(b, 100, 1))
	// 21 perfect moves and a black blunder
	cps = fill(21, 15)
	cps = append(cps, 900)
	w, b = gameAccuracy(cps)
	assert.True(t, isCloseTo(w, 100, 1))
	assert.True(t, isCloseTo(b, 50, 5))
	// 5 average moves (65 cpl) on each side
	cps = []int{-50, 15, -50, 15, -50, 15, -50, 15, -50, 15}
	w, b = gameAccuracy(cps)
	assert.True(t, isCloseTo(w, 76, 8))
	assert.True(t, isCloseTo(b, 76, 8))
	// 50 average moves (65 cpl) on each side
	cps = fill1(50, []int{-50, 15})
	w, b = gameAccuracy(cps)
	assert.True(t, isCloseTo(w, 76, 8))
	assert.True(t, isCloseTo(b, 76, 8))
	// 50 mediocre moves (150 cpl) on each side
	cps = fill1(50, []int{-135, 15})
	w, b = gameAccuracy(cps)
	assert.True(t, isCloseTo(w, 54, 8))
	assert.True(t, isCloseTo(b, 54, 8))
	// 50 terrible moves (500 cpl) on each side
	cps = fill1(50, []int{-435, 15})
	w, b = gameAccuracy(cps)
	assert.True(t, isCloseTo(w, 20, 8))
	assert.True(t, isCloseTo(b, 20, 8))
}

func Test_gameAccuracy1(t *testing.T) {
	type args struct {
		cps []int
	}
	tests := []struct {
		name  string
		args  args
		want  float64
		want1 float64
	}{
		{
			name: "test1",
			args: args{
				cps: []int{48, 38, 36, 36, 25, 61, 42, 131, 82, 135, 109, 186, 152, 145, 134, 280, 287, 264, 278, 271, 271, 306, 246, 335, 311, 500, 87, 276, 284, 398, 196, 514, 520, 641, -144, -74, -156, -107, -94, 60, 47, 146, -33, 98, -483, 31, -174, 385, -152, 467, 515, 846, 874, 884, 874, 938, 885, 889, 935, 946, 831, 998, 1018, 1026, 1031, 1142, 935, 1062, 1096, 1123, 1112, 1123},
			},
			want:  61.61609601278025,
			want1: 59.024965435590794,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := gameAccuracy(tt.args.cps)
			assert.Equalf(t, tt.want, got, "gameAccuracy(%v)", tt.args.cps)
			assert.Equalf(t, tt.want1, got1, "gameAccuracy(%v)", tt.args.cps)
		})
	}
}

func Test_standardDeviation(t *testing.T) {
	type args struct {
		num []float64
	}
	tests := []struct {
		name string
		args args
		want float64
	}{
		{
			name: "",
			args: args{
				num: []float64{91.37426170872673, 37.04656899332343, 43.22998523826144, 36.02211364871788, 40.27589492090241, 41.43247132264411, 55.50076484124536},
			},
			want: 18.186779319663053,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, standardDeviation(tt.args.num), "standardDeviation(%v)", tt.args.num)
		})
	}
}
