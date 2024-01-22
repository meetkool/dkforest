package poker

import (
	"dkforest/pkg/database"
	"time"
)

type PokerEvent struct {
	ID int
	// ID1 is the ID of the next card in the stack.
	// We need to pre-set it's z-index because z-index and transform are not working properly together.
	// And the z-index is actually set once the transform is completed.
	// So this hack makes it look like the next card from the stack move over the other cards...
	ID1    int
	ZIdx   int
	Name   string
	Top    int
	Left   int
	Reveal bool
	Angle  string
	UserID database.UserID
}

type GameStartedEvent struct {
	DealerSeatIdx int
}

type GameIsOverEvent struct{}

type GameIsDoneEvent struct {
	Winner     string
	WinnerHand string
}

type ResetCardsEvent struct {
}

type CashBonusEvent struct {
	PlayerSeatIdx int
	Amount        database.PokerChip
	Animation     bool
	IsGain        bool
}

type PlayerBetEvent struct {
	PlayerSeatIdx int
	Player        database.Username
	Bet           database.PokerChip
	TotalBet      database.PokerChip
	Cash          database.PokerChip
}

type RefreshLoadingIconEvent struct{}

type LogEvent struct {
	Message string
}

type AutoActionEvent struct {
	Message string
}

type RefreshButtonsEvent struct{}

type ErrorMsgEvent struct {
	Message string
}

func NewErrorMsgEvent(msg string) ErrorMsgEvent {
	return ErrorMsgEvent{Message: msg}
}

type PlayerFoldEvent struct {
	Card1Idx, Card2Idx int
}

type PokerMinRaiseUpdatedEvent struct {
	MinRaise database.PokerChip
}

type PokerMainPotUpdatedEvent struct {
	MainPot database.PokerChip
}

type PokerWaitTurnEvent struct {
	Idx       int
	CreatedAt time.Time
}

type RedrawSeatsEvent struct{}

type PokerSeatTakenEvent struct {
}

type PokerSeatLeftEvent struct {
}

type PokerYourTurnEvent struct {
}
