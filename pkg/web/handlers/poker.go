package handlers

import (
	"bytes"
	"dkforest/pkg/cache"
	"dkforest/pkg/config"
	"dkforest/pkg/database"
	dutils "dkforest/pkg/database/utils"
	"dkforest/pkg/pubsub"
	"dkforest/pkg/utils"
	"dkforest/pkg/web/handlers/poker"
	hutils "dkforest/pkg/web/handlers/utils"
	"dkforest/pkg/web/handlers/utils/stream"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/asaskevich/govalidator"
	"github.com/labstack/echo"
	wallet1 "github.com/monero-ecosystem/go-monero-rpc-client/wallet"
	"github.com/sirupsen/logrus"
	"github.com/teris-io/shortid"
	"image"
	"image/png"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

var pokerWithdrawCache = cache.NewWithKey[database.UserID, int64](10*time.Minute, time.Hour)

func PokerHomeHandler(c echo.Context) error {
	db := c.Get("database").(*database.DkfDB)
	authUser := c.Get("authUser").(*database.User)
	getImgStr := func(img image.Image) string {
		buf := bytes.NewBuffer([]byte(""))
		_ = png.Encode(buf, img)
		return base64.StdEncoding.EncodeToString(buf.Bytes())
	}
	if authUser.PokerXmrSubAddress == "" {
		if resp, err := config.Xmr().CreateAddress(&wallet1.RequestCreateAddress{}); err == nil {
			authUser.SetPokerXmrSubAddress(db, resp.Address)
		}
	}
	const minWithdrawAmount = 1
	var data pokerData
	data.RakeBackPct = poker.RakeBackPct * 100
	data.XmrPrice = fmt.Sprintf("$%.2f", config.MoneroPrice.Load())
	data.Transactions, _ = db.GetUserPokerXmrTransactions(authUser.ID)
	data.PokerXmrSubAddress = authUser.PokerXmrSubAddress
	data.RakeBack = authUser.PokerRakeBack
	data.ChipsTest = authUser.ChipsTest
	data.XmrBalance = authUser.XmrBalance
	withdrawUnique := rand.Int63()
	data.WithdrawUnique = withdrawUnique
	withdrawUniqueOrig, _ := pokerWithdrawCache.Get(authUser.ID)
	pokerWithdrawCache.SetD(authUser.ID, withdrawUnique)
	pokerTables, _ := db.GetPokerTables()
	pxmr := database.Piconero(0)
	data.HelperXmr = pxmr.XmrStr()
	data.HelperChips = pxmr.ToPokerChip()
	data.HelperpXmr = pxmr.RawString()
	data.HelperUsd = pxmr.UsdStr()
	userTableAccounts, _ := db.GetPokerTableAccounts(authUser.ID)
	for _, t := range pokerTables {
		var nbSeated int
		if g := poker.PokerInstance.GetGame(poker.RoomID(t.Slug)); g != nil {
			nbSeated = g.CountSeated()
		}
		tableBalance := database.PokerChip(0)
		for _, a := range userTableAccounts {
			if a.PokerTableID == t.ID {
				tableBalance = a.Amount
				break
			}
		}
		data.Tables = append(data.Tables, TmpTable{PokerTable: t, NbSeated: nbSeated, TableBalance: tableBalance})
	}

	if authUser.PokerXmrSubAddress != "" {
		b, _ := authUser.GetImage()
		data.Img = getImgStr(b)
	}

	if c.Request().Method == http.MethodGet {
		return c.Render(http.StatusOK, "poker", data)
	}

	formName := c.Request().PostFormValue("form_name")
	if formName == "helper" {
		data.HelperAmount = c.Request().PostFormValue("amount")
		data.HelperType = c.Request().PostFormValue("type")
		switch data.HelperType {
		case "usd":
			amount := utils.DoParseF64(data.HelperAmount)
			pxmr = database.Piconero(amount / config.MoneroPrice.Load() * 1_000_000_000_000)
		case "xmr":
			amount := utils.DoParseF64(data.HelperAmount)
			pxmr = database.Piconero(amount * 1_000_000_000_000)
		case "pxmr":
			amount := utils.DoParseUint64(data.HelperAmount)
			pxmr = database.Piconero(amount)
		case "chips":
			amount := utils.DoParseUint64(data.HelperAmount)
			chips := database.PokerChip(amount)
			pxmr = chips.ToPiconero()
		}
		data.HelperXmr = pxmr.XmrStr()
		data.HelperChips = pxmr.ToPokerChip()
		data.HelperpXmr = pxmr.RawString()
		data.HelperUsd = pxmr.UsdStr()
		return c.Render(http.StatusOK, "poker", data)
	}

	if formName == "join_table" {
		pokerTableSlug := c.Request().PostFormValue("table_slug")
		playerBuyIn := database.PokerChip(utils.DoParseUint64(c.Request().PostFormValue("buy_in")))
		if err := doJoinTable(db, pokerTableSlug, playerBuyIn, authUser.ID); err != nil {
			data.ErrorTable = err.Error()
			return c.Render(http.StatusOK, "poker", data)
		}
		return c.Redirect(http.StatusFound, "/poker/"+pokerTableSlug)

	} else if formName == "cash_out" {
		pokerTableSlug := c.Request().PostFormValue("table_slug")
		if err := doCashOut(db, pokerTableSlug, authUser.ID); err != nil {
			data.ErrorTable = err.Error()
			return c.Render(http.StatusOK, "poker", data)
		}
		return c.Redirect(http.StatusFound, "/poker")

	} else if formName == "reset_chips" {
		authUser.ResetChipsTest(db)
		return hutils.RedirectReferer(c)

	} else if formName == "claim_rake_back" {
		if err := db.ClaimRakeBack(authUser.ID); err != nil {
			logrus.Error(err)
		}
		return hutils.RedirectReferer(c)
	}

	if config.PokerWithdrawEnabled.IsFalse() {
		data.Error = "withdraw temporarily disabled"
		return c.Render(http.StatusOK, "poker", data)
	}

	withdrawAmount := database.Piconero(utils.DoParseUint64(c.Request().PostFormValue("withdraw_amount")))
	data.WithdrawAmount = withdrawAmount
	data.WithdrawAddress = c.Request().PostFormValue("withdraw_address")
	withdrawUniqueSub := utils.DoParseInt64(c.Request().PostFormValue("withdraw_unique"))

	if withdrawUniqueOrig == 0 || withdrawUniqueSub != withdrawUniqueOrig {
		data.Error = "form submitted twice, try again"
		return c.Render(http.StatusOK, "poker", data)
	}
	if len(data.WithdrawAddress) != 95 {
		data.Error = "invalid xmr address"
		return c.Render(http.StatusOK, "poker", data)
	}
	if !govalidator.Matches(data.WithdrawAddress, `^[0-9][0-9AB][123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz]{93}$`) {
		data.Error = "invalid xmr address"
		return c.Render(http.StatusOK, "poker", data)
	}
	if data.WithdrawAddress == authUser.PokerXmrSubAddress {
		data.Error = "cannot withdraw to the deposit address"
		return c.Render(http.StatusOK, "poker", data)
	}
	if withdrawAmount < minWithdrawAmount {
		data.Error = fmt.Sprintf("minimum withdraw amount is %d", minWithdrawAmount)
		return c.Render(http.StatusOK, "poker", data)
	}
	userBalance := authUser.XmrBalance
	if withdrawAmount > userBalance {
		data.Error = fmt.Sprintf("maximum withdraw amount is %d (%d)", userBalance, withdrawAmount)
		return c.Render(http.StatusOK, "poker", data)
	}
	withdrawAmount = utils.Clamp(withdrawAmount, minWithdrawAmount, userBalance)

	lastOutTransaction, _ := db.GetLastUserWithdrawPokerXmrTransaction(authUser.ID)
	if time.Since(lastOutTransaction.CreatedAt) < 5*time.Minute {
		diff := time.Until(lastOutTransaction.CreatedAt.Add(5 * time.Minute))
		data.Error = fmt.Sprintf("Wait %s before doing a new withdraw transaction", utils.ShortDur(diff))
		return c.Render(http.StatusOK, "poker", data)
	}

	walletRpcClient := config.Xmr()

	res, err := walletRpcClient.Transfer(&wallet1.RequestTransfer{
		DoNotRelay:    true,
		GetTxMetadata: true,
		Destinations: []*wallet1.Destination{
			{Address: data.WithdrawAddress,
				Amount: uint64(withdrawAmount)}}})
	if err != nil {
		logrus.Error(err)
		data.Error = err.Error()
		return c.Render(http.StatusOK, "poker", data)
	}

	transactionFee := database.Piconero(res.Fee)

	if withdrawAmount+transactionFee > authUser.XmrBalance {
		data.Error = fmt.Sprintf("not enough funds to pay for transaction fee %d (%s xmr)", transactionFee, transactionFee.XmrStr())
		return c.Render(http.StatusOK, "poker", data)
	}

	dutils.RootAdminNotify(db, fmt.Sprintf("new withdraw %s xmr by %s", withdrawAmount.XmrStr(), authUser.Username))

	var pokerXmrTx database.PokerXmrTransaction
	if err := db.WithE(func(tx *database.DkfDB) error {
		xmrBalance, err := authUser.GetXmrBalance(tx)
		if err != nil {
			return err
		}
		if withdrawAmount+transactionFee > xmrBalance {
			return errors.New("not enough funds")
		}
		if err := authUser.SubXmrBalance(tx, withdrawAmount+transactionFee); err != nil {
			return err
		}
		if pokerXmrTx, err = tx.CreatePokerXmrTransaction(authUser.ID, res); err != nil {
			logrus.Error("failed to create poker xmr transaction", err)
			return err
		}
		return nil
	}); err != nil {
		data.Error = err.Error()
		return c.Render(http.StatusOK, "poker", data)
	}

	if _, err := walletRpcClient.RelayTx(&wallet1.RequestRelayTx{Hex: res.TxMetadata}); err != nil {
		if err := db.WithE(func(tx *database.DkfDB) error {
			if err := pokerXmrTx.SetStatus(tx, database.PokerXmrTransactionStatusFailed); err != nil {
				return err
			}
			if err := authUser.IncrXmrBalance(tx, withdrawAmount+transactionFee); err != nil {
				return err
			}
			return nil
		}); err != nil {
			logrus.Error(err)
		}
		logrus.Error(err)
		data.Error = err.Error()
		return c.Render(http.StatusOK, "poker", data)
	}

	if err := pokerXmrTx.SetStatus(db, database.PokerXmrTransactionStatusSuccess); err != nil {
		logrus.Error(err)
	}

	pokerWithdrawCache.Delete(authUser.ID)
	return hutils.RedirectReferer(c)
}

func doJoinTable(db *database.DkfDB, pokerTableSlug string, playerBuyIn database.PokerChip, userID database.UserID) error {
	err := db.WithE(func(tx *database.DkfDB) error {
		roomID := poker.RoomID(pokerTableSlug)
		g := poker.PokerInstance.GetGame(roomID)
		if g == nil {
			pokerTable, err := tx.GetPokerTableBySlug(pokerTableSlug)
			if err != nil {
				return errors.New("failed to get poker table")
			}
			g = poker.PokerInstance.CreateGame(db, roomID, pokerTable.ID, pokerTable.MinBet, pokerTable.IsTest)
		}
		g.Players.Lock()
		defer g.Players.Unlock()
		if g.IsSeatedUnsafe(userID) {
			return errors.New("cannot buy-in while seated")
		}
		pokerTable, err := tx.GetPokerTableBySlug(pokerTableSlug)
		if err != nil {
			return errors.New("table mot found")
		}
		if playerBuyIn < pokerTable.MinBuyIn {
			return errors.New("buy in too small")
		}
		if playerBuyIn > pokerTable.MaxBuyIn {
			return errors.New("buy in too high")
		}
		xmrBalance, chipsTestBalance, err := tx.GetUserBalances(userID)
		if err != nil {
			return errors.New("failed to get user's balance")
		}
		userChips := utils.Ternary(pokerTable.IsTest, chipsTestBalance, xmrBalance.ToPokerChip())
		if userChips < playerBuyIn {
			return errors.New("not enough chips to buy-in")
		}
		tableAccount, err := tx.GetPokerTableAccount(userID, pokerTable.ID)
		if err != nil {
			return errors.New("failed to get table account")
		}
		if tableAccount.Amount+playerBuyIn > pokerTable.MaxBuyIn {
			return errors.New("buy-in exceed table max buy-in")
		}
		tableAccount.Amount += playerBuyIn
		if err := tx.DecrUserBalance(userID, pokerTable.IsTest, playerBuyIn); err != nil {
			return errors.New("failed to update user's balance")
		}
		if err := tableAccount.Save(tx); err != nil {
			return errors.New("failed to update user's table account")
		}
		return nil
	})
	return err
}

func doCashOut(db *database.DkfDB, pokerTableSlug string, userID database.UserID) error {
	err := db.WithE(func(tx *database.DkfDB) error {
		roomID := poker.RoomID(pokerTableSlug)
		g := poker.PokerInstance.GetGame(roomID)
		if g == nil {
			pokerTable, err := tx.GetPokerTableBySlug(pokerTableSlug)
			if err != nil {
				return errors.New("failed to get poker table")
			}
			g = poker.PokerInstance.CreateGame(db, roomID, pokerTable.ID, pokerTable.MinBet, pokerTable.IsTest)
		}
		g.Players.Lock()
		defer g.Players.Unlock()
		if g.IsSeatedUnsafe(userID) {
			return errors.New("cannot cash out while seated")
		}
		pokerTable, err := tx.GetPokerTableBySlug(pokerTableSlug)
		if err != nil {
			return errors.New("table mot found")
		}
		account, err := tx.GetPokerTableAccount(userID, pokerTable.ID)
		if err != nil {
			return errors.New("failed to get table account")
		}
		if err := tx.IncrUserBalance(userID, pokerTable.IsTest, account.Amount); err != nil {
			return errors.New("failed to update user's balance")
		}
		account.Amount = 0
		if err := account.Save(tx); err != nil {
			return errors.New("failed to update user's table account")
		}
		return nil
	})
	return err
}

func PokerRakeBackHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)

	var data pokerRakeBackData
	data.RakeBackPct = poker.RakeBackPct * 100
	data.ReferredCount, _ = db.GetRakeBackReferredCount(authUser.ID)
	pokerReferralToken := authUser.PokerReferralToken
	if pokerReferralToken != nil {
		data.ReferralToken = *pokerReferralToken
		data.ReferralURL = fmt.Sprintf("%s/poker?r=%s", config.DkfOnion, *pokerReferralToken)
	}

	if c.Request().Method == http.MethodGet {
		return c.Render(http.StatusOK, "poker-rake-back", data)
	}

	formName := c.Request().PostFormValue("form_name")
	if formName == "generate_referral_url" {
		if pokerReferralToken != nil {
			return hutils.RedirectReferer(c)
		}
		token, err := shortid.Generate()
		if err != nil {
			logrus.Error(err)
			return hutils.RedirectReferer(c)
		}
		authUser.SetPokerReferralToken(db, &token)
		return hutils.RedirectReferer(c)

	} else if formName == "set_referrer" {
		referralToken := c.Request().PostFormValue("referral_token")
		if len(referralToken) != 9 {
			data.SetReferralError = "Invalid referral token"
			return c.Render(http.StatusOK, "poker-rake-back", data)
		}
		if authUser.PokerReferredBy != nil {
			data.SetReferralError = "You are already giving your rake back"
			return c.Render(http.StatusOK, "poker-rake-back", data)
		}
		if pokerReferralToken != nil && referralToken == *pokerReferralToken {
			data.SetReferralError = "Yon can't give yourself the rake back"
			return c.Render(http.StatusOK, "poker-rake-back", data)
		}
		referrer, err := db.GetUserByPokerReferralToken(referralToken)
		if err != nil {
			data.SetReferralError = "no user found with this referral token"
			return c.Render(http.StatusOK, "poker-rake-back", data)
		}
		if referrer.ID == authUser.ID {
			data.SetReferralError = "Yon can't give yourself the rake back"
			return c.Render(http.StatusOK, "poker-rake-back", data)
		}
		authUser.SetPokerReferredBy(db, &referrer.ID)
		return hutils.RedirectReferer(c)
	}
	return hutils.RedirectReferer(c)
}

func PokerTableHandler(c echo.Context) error {
	roomID := c.Param("roomID")
	var data pokerTableData
	data.PokerTableSlug = roomID
	return c.Render(http.StatusOK, "poker-table", data)
}

func PokerStreamHandler(c echo.Context) error {
	roomID := poker.RoomID(c.Param("roomID"))
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)

	pokerTable, err := db.GetPokerTableBySlug(roomID.String())
	if err != nil {
		return c.Redirect(http.StatusFound, "/")
	}

	chatRoomSlug := "general"
	tmp := strings.ReplaceAll(roomID.String(), "-", "_")
	if _, err := db.GetChatRoomByName(tmp); err == nil {
		chatRoomSlug = tmp
	}

	roomTopic := roomID.Topic()
	roomUserTopic := roomID.UserTopic(authUser.ID)
	send := func(s string) { _, _ = c.Response().Write([]byte(s)) }

	g := poker.PokerInstance.GetOrCreateGame(db, roomID, pokerTable.ID, pokerTable.MinBet, pokerTable.IsTest)

	streamItem, err := stream.SetStreaming(c, authUser.ID, roomTopic)
	if err != nil {
		return nil
	}
	defer streamItem.Cleanup()

	sub := poker.PubSub.Subscribe([]string{roomTopic, roomUserTopic, "refresh_loading_icon_" + string(authUser.Username)})
	defer sub.Close()

	send(poker.BuildBaseHtml(g, authUser, chatRoomSlug))
	c.Response().Flush()

	loop(streamItem.Quit, sub, func(topic string, payload any) error {
		switch payload.(type) {
		case poker.RefreshLoadingIconEvent:
			send(hutils.MetaRefresh(1))
			return BreakLoopErr
		}
		send(poker.BuildPayloadHtml(g, authUser, payload))
		c.Response().Flush()
		return nil
	})

	return nil
}

func PokerLogsHandler(c echo.Context) error {
	roomID := poker.RoomID(c.Param("roomID"))
	authUser := c.Get("authUser").(*database.User)
	send := func(s string) { _, _ = c.Response().Write([]byte(s)) }
	g := poker.PokerInstance.GetGame(roomID)
	if g == nil {
		return c.Redirect(http.StatusFound, "/")
	}
	roomLogsTopic := roomID.LogsTopic()
	sub := poker.PubSub.Subscribe([]string{roomLogsTopic, "refresh_loading_icon_" + string(authUser.Username)})
	defer sub.Close()

	streamItem, err := stream.SetStreaming(c, authUser.ID, roomLogsTopic)
	if err != nil {
		return nil
	}
	defer streamItem.Cleanup()

	send(hutils.HtmlCssReset)
	send(`<style>body { background-color: #444; color: #ddd; padding: 3px; }</style><div style="display:flex;flex-direction:column-reverse;">`)
	for _, evt := range g.GetLogs() {
		send(fmt.Sprintf(`<div>%s</div>`, evt.Message))
	}
	c.Response().Flush()

	loop(streamItem.Quit, sub, func(topic string, payload any) error {
		switch evt := payload.(type) {
		case poker.RefreshLoadingIconEvent:
			send(hutils.MetaRefresh(1))
			return BreakLoopErr
		case poker.LogEvent:
			send(fmt.Sprintf(`<div>%s</div>`, evt.Message))
			c.Response().Flush()
		}
		return nil
	})

	return nil
}

func PokerBetHandler(c echo.Context) error {
	roomID := poker.RoomID(c.Param("roomID"))
	authUser := c.Get("authUser").(*database.User)
	send := func(s string) { _, _ = c.Response().Write([]byte(s)) }
	g := poker.PokerInstance.GetGame(roomID)
	if g == nil {
		return c.Redirect(http.StatusFound, "/")
	}

	roomUserTopic := roomID.UserTopic(authUser.ID)
	sub := poker.PubSub.Subscribe([]string{roomID.Topic(), roomUserTopic, "refresh_loading_icon_" + string(authUser.Username)})
	defer sub.Close()

	streamItem, err := stream.SetStreaming(c, authUser.ID, roomUserTopic)
	if err != nil {
		return nil
	}
	defer streamItem.Cleanup()

	if c.Request().Method == http.MethodPost {
		submitBtn := c.Request().PostFormValue("submitBtn")
		if submitBtn == "check" {
			g.Check(authUser.ID)
		} else if submitBtn == "call" {
			g.Call(authUser.ID)
		} else if submitBtn == "fold" {
			g.Fold(authUser.ID)
		} else if submitBtn == "allIn" {
			g.AllIn(authUser.ID)
		} else {
			raiseBtn := c.Request().PostFormValue("raise")
			if raiseBtn == "raise" {
				g.Raise(authUser.ID)
			} else if raiseBtn == "raiseValue" {
				raiseValue := database.PokerChip(utils.DoParseUint64(c.Request().PostFormValue("raiseValue")))
				g.Bet(authUser.ID, raiseValue)
			}
		}
		send(hutils.MetaRefreshNow())
		c.Response().Flush()
		return nil

	} else {

		send(hutils.HtmlCssReset)

		if player := g.OngoingPlayer(authUser.ID); player != nil {
			betBtnLbl := utils.Ternary(g.IsBet(), "Bet", "Raise")
			minRaise := g.MinRaise()
			canCheck := true
			canFold := true
			canCall := true
			if g.IsYourTurn(player) {
				playerBet := player.GetBet()
				minBet := g.MinBet()
				canCheck = g.CanCheck(player)
				canFold = g.CanFold(player)
				canCall = minBet-playerBet > 0
			}
			send(fmt.Sprintf(`
	<style>
		.raise-container {
			display: inline-block;
			margin-right: 20px;
		}
		.raise-input {
			width: 90px;
			-moz-appearance: textfield;
		}
		.raise-btn {
			width: 51px;
		}
		.button-container {
			display: inline-block;
			vertical-align: top;
		}
		.min-raise-text {
			margin-top: 4px;
			font-family: Arial, Helvetica, sans-serif;
			font-size: 18px;
		}
	</style>
	<div>
		<form method="post">
			<div class="raise-container">
				<input type="number" name="raiseValue" value="%s" min="%s" class="raise-input" />
				<button type="submit" name="raise" value="raiseValue" class="raise-btn">%s</button><br />
			</div>
			<div class="button-container">
				<button name="submitBtn" value="check" %s>Check</button>
				<button name="submitBtn" value="call" %s>Call</button>
				<button name="submitBtn" value="fold" %s>Fold</button>
				<button name="submitBtn" value="allIn">All-in</button>
			</div>
		</form>
		<div class="min-raise-text">Min raise: %d</div>
	</div>
`,
				minRaise, minRaise,
				betBtnLbl,
				utils.TernaryOrZero(!canCheck, "disabled"),
				utils.TernaryOrZero(!canCall, "disabled"),
				utils.TernaryOrZero(!canFold, "disabled"),
				minRaise))
		}
		c.Response().Flush()
	}

	loop(streamItem.Quit, sub, func(topic string, payload any) error {
		switch payload.(type) {
		case poker.RefreshLoadingIconEvent:
			send(hutils.MetaRefresh(1))
			return BreakLoopErr
		case poker.RefreshButtonsEvent:
			send(hutils.MetaRefreshNow())
			c.Response().Flush()
			return BreakLoopErr
		}
		return nil
	})

	return nil
}

var BreakLoopErr = errors.New("break Loop")
var ContinueLoopErr = errors.New("continue Loop")

func loop[T any](quit <-chan struct{}, sub *pubsub.Sub[T], clb func(topic string, payload T) error) {
Loop:
	for {
		select {
		case <-quit:
			break Loop
		default:
		}

		topic, payload, err := sub.ReceiveTimeout2(1*time.Second, quit)
		if err != nil {
			if errors.Is(err, pubsub.ErrCancelled) {
				break Loop
			}
			continue
		}

		if err := clb(topic, payload); err != nil {
			if errors.Is(err, BreakLoopErr) {
				break Loop
			} else if errors.Is(err, ContinueLoopErr) {
				continue Loop
			}
		}
	}
}

func PokerDealHandler(c echo.Context) error {
	roomID := poker.RoomID(c.Param("roomID"))
	authUser := c.Get("authUser").(*database.User)
	g := poker.PokerInstance.GetGame(roomID)
	if g == nil {
		return c.NoContent(http.StatusNotFound)
	}
	if c.Request().Method == http.MethodPost {
		g.Deal(authUser.ID)
	}
	html := hutils.HtmlCssReset
	html += `<form method="post"><button>Deal</button></form>`
	return c.HTML(http.StatusOK, html)
}

func PokerUnSitHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	roomID := poker.RoomID(c.Param("roomID"))
	html := hutils.HtmlCssReset + `<form method="post"><button>UnSit</button></form>`
	g := poker.PokerInstance.GetGame(roomID)
	if g == nil {
		return c.NoContent(http.StatusNotFound)
	}
	if c.Request().Method == http.MethodPost {
		g.UnSit(authUser.ID)
	}
	return c.HTML(http.StatusOK, html)
}

func PokerSitHandler(c echo.Context) error {
	html := hutils.HtmlCssReset + `<form method="post"><button style="height: 40px; width: 65px;" title="Take seat"><img src="/public/img/throne.png" width="30" alt="sit" /></button></form>`
	authUser := c.Get("authUser").(*database.User)
	pos := utils.Clamp(utils.DoParseInt(c.Param("pos")), 1, poker.NbPlayers) - 1
	roomID := poker.RoomID(c.Param("roomID"))
	g := poker.PokerInstance.GetGame(roomID)
	if g == nil {
		return c.HTML(http.StatusOK, html)
	}
	if c.Request().Method == http.MethodPost {
		g.Sit(authUser.ID, authUser.Username, authUser.PokerReferredBy, pos)
	}
	return c.HTML(http.StatusOK, html)
}
