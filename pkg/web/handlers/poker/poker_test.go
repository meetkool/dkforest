package poker

import (
	"dkforest/pkg/database"
	"dkforest/pkg/utils/rwmtx"
	"github.com/stretchr/testify/assert"
	"testing"
)

func n(v uint64) rwmtx.RWMtxUInt64[database.PokerChip] {
	return rwmtx.RWMtxUInt64[database.PokerChip]{rwmtx.New(database.PokerChip(v))}
}

func Test_sortGameResults(t *testing.T) {
	p1 := &PokerPlayer{seatedPlayer: &seatedPlayer{cash: n(0), username: "p1"}, gameBet: 10}
	p2 := &PokerPlayer{seatedPlayer: &seatedPlayer{cash: n(0), username: "p2"}, gameBet: 20}
	p3 := &PokerPlayer{seatedPlayer: &seatedPlayer{cash: n(0), username: "p3"}, gameBet: 30}
	p4 := &PokerPlayer{seatedPlayer: &seatedPlayer{cash: n(1), username: "p4"}, gameBet: 100}
	arr := []gameResult{
		{1, []*PokerPlayer{p2, p4, p1, p3}},
	}
	sortGameResults(arr)
	assert.Equal(t, database.Username("p1"), arr[0].players[0].username)
	assert.Equal(t, database.Username("p2"), arr[0].players[1].username)
	assert.Equal(t, database.Username("p3"), arr[0].players[2].username)
	assert.Equal(t, database.Username("p4"), arr[0].players[3].username)
}

func Test_processPot(t *testing.T) {
	var p1, p2, p3, p4 *PokerPlayer
	var arr []gameResult
	var res []PlayerGain

	p1 = &PokerPlayer{seatedPlayer: &seatedPlayer{cash: n(0), username: "p1"}, gameBet: 100, allInMaxGain: 400}
	p2 = &PokerPlayer{seatedPlayer: &seatedPlayer{cash: n(0), username: "p2"}, gameBet: 200, allInMaxGain: 700}
	p3 = &PokerPlayer{seatedPlayer: &seatedPlayer{cash: n(0), username: "p3"}, gameBet: 300, allInMaxGain: 900}
	p4 = &PokerPlayer{seatedPlayer: &seatedPlayer{cash: n(1), username: "p4"}, gameBet: 400}
	arr = []gameResult{
		{1, []*PokerPlayer{p2, p4, p1, p3}},
	}
	sortGameResults(arr)
	res, _ = processPot(arr, 1000, 20, false, 4)
	assert.Equal(t, database.PokerChip(250), res[0].Gain)
	assert.Equal(t, database.PokerChip(250), res[1].Gain)
	assert.Equal(t, database.PokerChip(250), res[2].Gain)
	assert.Equal(t, database.PokerChip(250), res[3].Gain)

	p1 = &PokerPlayer{seatedPlayer: &seatedPlayer{cash: n(0), username: "p1"}, gameBet: 10, allInMaxGain: 40}
	p2 = &PokerPlayer{seatedPlayer: &seatedPlayer{cash: n(0), username: "p2"}, gameBet: 20, allInMaxGain: 70}
	p3 = &PokerPlayer{seatedPlayer: &seatedPlayer{cash: n(0), username: "p3"}, gameBet: 300, allInMaxGain: 630}
	p4 = &PokerPlayer{seatedPlayer: &seatedPlayer{cash: n(1), username: "p4"}, gameBet: 400}
	arr = []gameResult{
		{1, []*PokerPlayer{p2, p4, p1, p3}},
	}
	sortGameResults(arr)
	res, _ = processPot(arr, 1000, 20, false, 4)
	assert.Equal(t, database.PokerChip(40), res[0].Gain)
	assert.Equal(t, database.PokerChip(70), res[1].Gain)
	assert.Equal(t, database.PokerChip(445), res[2].Gain)
	assert.Equal(t, database.PokerChip(445), res[3].Gain)

	p1 = &PokerPlayer{seatedPlayer: &seatedPlayer{cash: n(1), username: "p1"}, gameBet: 500}
	p2 = &PokerPlayer{seatedPlayer: &seatedPlayer{cash: n(0), username: "p2"}, gameBet: 500, allInMaxGain: 1000}
	arr = []gameResult{
		{1, []*PokerPlayer{p2}},
		{2, []*PokerPlayer{p1}},
	}
	sortGameResults(arr)
	res, _ = processPot(arr, 1000, 20, false, 2)
	assert.Equal(t, 1, len(res))
	assert.Equal(t, database.Username("p2"), res[0].Player.username)
	assert.Equal(t, database.PokerChip(1000), res[0].Gain)

	p1 = &PokerPlayer{seatedPlayer: &seatedPlayer{cash: n(1), username: "p1"}, gameBet: 5}
	p2 = &PokerPlayer{seatedPlayer: &seatedPlayer{cash: n(1), username: "p2"}, gameBet: 5}
	p3 = &PokerPlayer{seatedPlayer: &seatedPlayer{cash: n(1), username: "p3"}, gameBet: 5}
	p4 = &PokerPlayer{seatedPlayer: &seatedPlayer{cash: n(1), username: "p4"}, gameBet: 5}
	//p5 = &PokerPlayer{Cash: 1, GameBet: 3, Folded: true, Username: "p5"}
	arr = []gameResult{
		{1, []*PokerPlayer{p1, p2, p3}},
		{2, []*PokerPlayer{p4}},
	}
	sortGameResults(arr)
	res, _ = processPot(arr, 23, 20, false, 4)
	assert.Equal(t, 3, len(res))
	assert.Equal(t, database.PokerChip(8), res[0].Gain)
	assert.Equal(t, database.PokerChip(8), res[1].Gain)
	assert.Equal(t, database.PokerChip(7), res[2].Gain)

	p1 = &PokerPlayer{seatedPlayer: &seatedPlayer{cash: n(0), username: "p1"}, gameBet: 900, allInMaxGain: 1560}
	p2 = &PokerPlayer{seatedPlayer: &seatedPlayer{cash: n(0), username: "p2"}, gameBet: 640, allInMaxGain: 1300}
	arr = []gameResult{
		{1, []*PokerPlayer{p2}},
		{2, []*PokerPlayer{p1}},
	}
	sortGameResults(arr)
	res, _ = processPot(arr, 1560, 20, false, 2)
	assert.Equal(t, 2, len(res))
	assert.Equal(t, database.PokerChip(1300), res[0].Gain)
	assert.Equal(t, database.PokerChip(260), res[1].Gain)

	p1 = &PokerPlayer{seatedPlayer: &seatedPlayer{cash: n(1), username: "p1"}, gameBet: 500}
	arr = []gameResult{
		{2, []*PokerPlayer{p1}},
	}
	sortGameResults(arr)
	res, _ = processPot(arr, 1000, 20, false, 2)
	assert.Equal(t, 1, len(res))
	assert.Equal(t, database.Username("p1"), res[0].Player.username)
	assert.Equal(t, database.PokerChip(1000), res[0].Gain)
	assert.Equal(t, "Only player alive", res[0].HandStr)
}

func Test_isRoundSettled(t *testing.T) {
	type args struct {
		players []*PokerPlayer
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{"1", args{players: []*PokerPlayer{
			{bet: n(10), seatedPlayer: &seatedPlayer{cash: n(0)}},
			{bet: n(20), seatedPlayer: &seatedPlayer{cash: n(0)}},
			{bet: n(30), seatedPlayer: &seatedPlayer{cash: n(1)}},
			{bet: n(30), seatedPlayer: &seatedPlayer{cash: n(1)}}}}, true},
		{"2", args{players: []*PokerPlayer{
			{bet: n(100), seatedPlayer: &seatedPlayer{cash: n(0)}},
			{bet: n(20), seatedPlayer: &seatedPlayer{cash: n(0)}},
			{bet: n(30), seatedPlayer: &seatedPlayer{cash: n(1)}},
			{bet: n(30), seatedPlayer: &seatedPlayer{cash: n(1)}}}}, false},
		{"3", args{players: []*PokerPlayer{
			{bet: n(10), seatedPlayer: &seatedPlayer{cash: n(0)}},
			{bet: n(200), seatedPlayer: &seatedPlayer{cash: n(0)}},
			{bet: n(30), seatedPlayer: &seatedPlayer{cash: n(1)}},
			{bet: n(30), seatedPlayer: &seatedPlayer{cash: n(1)}}}}, false},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, isRoundSettled(tt.args.players), "isRoundSettled(%v)", tt.args.players)
		})
	}
}
