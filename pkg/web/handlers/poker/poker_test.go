package poker

import (
	"container/list"
	"dkforest/pkg/database"
	"dkforest/pkg/utils/rwmtx"
	"github.com/stretchr/testify/assert"
	"math"
	"sort"
)

type PlayerGain struct {
	Player   *PokerPlayer
	Gain     database.PokerChip
	HandStr  string
}

type gameResult struct {
	roundNumber int
	players     []*PokerPlayer
}

func (g *gameResult) totalBet() database.PokerChip {
	total := database.PokerChip(0)
	for _, p := range g.players {
		total += p.bet.Load()
	}
	return total
}

func sortGameResults(arr *[]gameResult) {
	sort.Slice(arr, func(i, j int) bool {
		return (*arr)[i].roundNumber < (*arr)[j].roundNumber
	})
}

func processPot(arr *[]gameResult, potSize database.PokerChip, rakePercent float64, isFinalRound bool, numPlayers int) ([]PlayerGain, database.PokerChip) {
	var res []PlayerGain
	remainingPot := potSize
	for _, g := range *arr {
		if isFinalRound || g.totalBet() == remainingPot {
			betPerPlayer := rakePercent * g.totalBet() / 100
			rake := database.PokerChip(math.Ceil(float64(betPerPlayer) / float64(numPlayers)))
			remainingPot -= rake
			for _, p := range g.players {
				gain := database.PokerChip(0)
				if p.bet.Load() > 0 {
					gain = remainingPot / database.PokerChip(len(g.players))
					if p.allInMaxGain > 0 {
						gain = min(p.allInMaxGain, gain)
					}
				}
				res = append(res, PlayerGain{p, gain, p.handStr()})
			}
			return res, rake
		}
		remainingPot -= g.totalBet()
	}
	return nil, potSize
}

func min(a, b database.PokerChip) database.PokerChip {
	if a < b {
		return a
	}
	return b
}

func Test_sortGameResults(t *testing.T) {
	p1 := &PokerPlayer{seatedPlayer: &seatedPlayer{cash: n(0), username: "p1"}, gameBet: 10}
	p2 := &PokerPlayer{seatedPlayer: &seatedPlayer{cash: n(0), username: "p2"}, gameBet: 20}
	p3 := &PokerPlayer{seatedPlayer: &seatedPlayer{cash: n(0), username: "p3"}, gameBet: 30}
	p4 := &PokerPlayer{seatedPlayer: &seatedPlayer{cash: n(1), username: "p4"}, gameBet: 100}
	arr := []gameResult{
		{1, []*PokerPlayer{p2, p4, p1, p3}},
	}
	sortGameResults(&arr)
	assert.Equal(t, "p1", arr[0].players[0].username)
	assert.Equal(t, "p2", arr[0].players[1].username)
	assert.Equal(t, "p3", arr[0].players[2].username)
	assert.Equal(t, "p4", arr[0].players[3].username)
}

// ... (rest of the test cases remain the same)

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
		// ... (rest of the test cases remain the same)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, isRoundSettled(tt.args.players), "isRoundSettled(%v)", tt.args.players)
		})
	}
}

