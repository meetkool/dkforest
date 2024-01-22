package interceptors

import (
	"dkforest/pkg/config"
	"dkforest/pkg/database"
	dutils "dkforest/pkg/database/utils"
	"dkforest/pkg/utils"
	"dkforest/pkg/web/handlers/interceptors/command"
	"errors"
	"github.com/sirupsen/logrus"
	"regexp"
	"strings"
	"sync"
	"time"
)

type SpamInterceptor struct{}

type Filter struct {
	IsRegex bool
	Term    string
	Rgx     *regexp.Regexp
	Kick    bool
	Hb      bool
}

var filters []Filter
var filtersMtx sync.RWMutex

func LoadFilters(db *database.DkfDB) {
	filtersMtx.Lock()
	defer filtersMtx.Unlock()
	filters = make([]Filter, 0)
	dbFilters, _ := db.GetSpamFilters()
	for _, dbFilter := range dbFilters {
		f := Filter{IsRegex: dbFilter.IsRegex}
		if dbFilter.Action == 1 {
			f.Kick = true
		} else if dbFilter.Action == 2 {
			f.Hb = true
		}
		if dbFilter.IsRegex {
			f.Rgx = regexp.MustCompile(dbFilter.Filter)
		} else {
			f.Term = dbFilter.Filter
		}
		filters = append(filters, f)
	}
}

// Check the filters that we have in the database.
func checkDynamicFilters(c *command.Command, lowerCaseMessage string, silentSelfKick bool) error {
	filtersMtx.RLock()
	defer filtersMtx.RUnlock()
	for _, f := range filters {
		isMatch := (f.IsRegex && f.Rgx.MatchString(c.Message)) ||
			(!f.IsRegex && strings.Contains(lowerCaseMessage, f.Term))
		if isMatch {
			if f.Hb {
				dutils.SelfHellBan(c.DB, c.AuthUser)
				return ErrSilent
			}
			if f.Kick {
				_ = dutils.SelfKick(c.DB, *c.AuthUser, silentSelfKick)
			}
			return ErrSpamFilterTriggered
		}
	}
	return nil
}

func (i SpamInterceptor) InterceptMsg(c *command.Command) {
	lowerCaseMessage := strings.ToLower(c.Message)
	silentSelfKick := config.SilentSelfKick.Load()

	if err := checkDynamicFilters(c, lowerCaseMessage, silentSelfKick); err != nil {
		if !errors.Is(err, ErrSilent) {
			c.Err = err
		}
		return
	}

	if c.Room.IsOfficialRoom() {
		if err := checkSpam(c.DB, c.OrigMessage, lowerCaseMessage, c.AuthUser); err != nil {
			c.Err = err
			return
		}
	}

	// Check CP links
	if checkCPLinks(c.DB, c.Message) {
		c.Err = errors.New("forbidden url")
		return
	}

	if !c.AuthUser.CanUseUppercase {
		c.Message = strings.ToLower(c.Message)
	}
}

var ErrSilent = errors.New("")
var ErrSpamFilterTriggered = errors.New("spam filter triggered")

func checkSpam(db *database.DkfDB, origMessage, lowerCaseMessage string, authUser *database.User) error {
	silentSelfKick := config.SilentSelfKick.Load()

	// Kick retard new users
	if time.Since(authUser.CreatedAt) < 5*time.Hour {
		if strings.Contains(lowerCaseMessage, "fucked up links") ||
			strings.Contains(lowerCaseMessage, "i wanna see gore") ||
			strings.Contains(lowerCaseMessage, "how can i make money") ||
			strings.Contains(lowerCaseMessage, "any links for scary stuff") {
			_ = dutils.SelfKick(db, *authUser, silentSelfKick)
			return ErrSpamFilterTriggered
		}
	}
	if authUser.GeneralMessagesCount < 20 || time.Since(authUser.CreatedAt) < 5*time.Hour {
		if strings.Contains(lowerCaseMessage, "cp link") {
			_ = dutils.SelfKick(db, *authUser, silentSelfKick)
			return ErrSpamFilterTriggered
		}
	}

	if strings.Contains(lowerCaseMessage, "#dorkforest") {
		if authUser.IsModerator() {
			return ErrSpamFilterTriggered
		}
		_ = dutils.SelfKick(db, *authUser, silentSelfKick)
		return ErrSpamFilterTriggered
	}

	// Auto kick upper case typing retards
	if authUser.GeneralMessagesCount <= 5 {
		count, total := utils.CountUppercase(origMessage)
		pct := float64(count) / float64(total)
		if total > 5 && pct > 0.8 {
			_ = dutils.SelfKick(db, *authUser, silentSelfKick)
			return ErrSpamFilterTriggered
		}
	}

	// Auto HB "new here"/"legit market" retards
	if autoHellbanCheck(authUser, lowerCaseMessage) {
		dutils.SelfHellBan(db, authUser)
		return nil
	}

	if autoKickSpammers(authUser, lowerCaseMessage) {
		_ = dutils.SelfKick(db, *authUser, silentSelfKick)
		return ErrSpamFilterTriggered
	}

	tot, wordsMap := utils.WordCount(lowerCaseMessage)
	if tot >= 5 {
		totalUniqueWords := len(wordsMap)
		uniqueRatio := float64(totalUniqueWords) / float64(tot)
		repeatedWordsCount := 0
		for word, count := range wordsMap {
			if len(word) >= 5 && count > 10 {
				repeatedWordsCount++
			}
		}
		retardRatio := float64(repeatedWordsCount) / float64(totalUniqueWords)
		//fmt.Println(tot, totalUniqueWords, uniqueRatio, repeatedWordsCount, retardRatio, wordsMap)
		if uniqueRatio < 0.2 {
			logrus.Error("failed unique ratio: " + utils.TruncStr(origMessage, 75, "…"))
			return errors.New("failed unique ratio")
		}
		if retardRatio > 0.1 {
			logrus.Error("failed retard ratio: " + utils.TruncStr(origMessage, 75, "…"))
			return errors.New("failed retard ratio")
		}
	}

	if authUser.GeneralMessagesCount < 10 {
		if autoKickProfanity(tot, wordsMap) {
			_ = dutils.SelfKick(db, *authUser, silentSelfKick)
			return ErrSpamFilterTriggered
		}
	}

	if authUser.GeneralMessagesCount < 4 {
		if (wordsMap["need"] > 0 && wordsMap["help"] > 0) ||
			(wordsMap["help"] > 0 && wordsMap["me"] > 0) ||
			(wordsMap["make"] > 0 && wordsMap["money"] > 0) ||
			(wordsMap["interesting"] > 0 && (wordsMap["link"] > 0 || wordsMap["links"] > 0)) ||
			wordsMap["porn"] > 0 ||
			wordsMap["pedo"] > 0 ||
			wordsMap["murder"] > 0 {
			_ = dutils.SelfKick(db, *authUser, silentSelfKick)
			return ErrSpamFilterTriggered
		}
	}

	if authUser.GeneralMessagesCount < 10 {
		if ((wordsMap["learn"] > 0 || wordsMap["teach"] > 0) && (wordsMap["hacking"] > 0 || wordsMap["hack"] > 0)) ||
			(wordsMap["cook"] > 0 && wordsMap["meth"] > 0) ||
			(wordsMap["creepy"] > 0 && (wordsMap["site"] > 0 || wordsMap["sites"] > 0)) ||
			(wordsMap["porn"] > 0 && (wordsMap["link"] > 0 || wordsMap["links"] > 0)) ||
			(wordsMap["topic"] > 0 && wordsMap["link"] > 0) {
			_ = dutils.SelfKick(db, *authUser, silentSelfKick)
			return ErrSpamFilterTriggered
		}
	}

	if authUser.GeneralMessagesCount < 20 || time.Since(authUser.CreatedAt) < 5*time.Hour {
		if wordsMap["cp"] > 0 && (wordsMap["link"] > 0 || wordsMap["links"] > 0) {
			_ = dutils.SelfKick(db, *authUser, silentSelfKick)
			return ErrSpamFilterTriggered
		}
	}

	return nil
}

func autoKickProfanityTmp(orig string) bool {
	tot, m := utils.WordCount(strings.ToLower(orig))
	return autoKickProfanity(tot, m)
}

func autoKickProfanity(tot int, wordsMap map[string]int) bool {
	if tot > 4 && countProfanity(wordsMap) >= 4 {
		return true
	}
	return false
}

func countProfanity(wordsMap map[string]int) int {
	profanityWords := []string{"anus", "asshole", "cock", "dick", "nigger", "niggers", "nigga", "niggas", "sex", "rape", "porn",
		"cunt", "murder", "fuck", "blood", "corpse", "hole", "slut", "bitch", "shit", "poop", "butt", "faggot",
		"submissive", "slurping", "suck", "nuts", "gore", "stupid", "dumb", "jerking", "rotten", "rotted", "stinky"}
	profanity := 0
	for _, w := range profanityWords {
		if n, ok := wordsMap[w]; ok {
			profanity += n
		}
	}
	return profanity
}

var spamCharsRgx = regexp.MustCompile("[^a-z0-9]+")

func autoKickSpammers(authUser *database.User, lowerCaseMessage string) bool {
	if authUser.GeneralMessagesCount <= 10 {
		processedString := spamCharsRgx.ReplaceAllString(lowerCaseMessage, "")
		return strings.Contains(processedString, "lemybeauty") ||
			strings.Contains(processedString, "blacktorcc") ||
			strings.Contains(processedString, "profjerry") ||
			strings.Contains(processedString, "shopdarkse")
	}
	return false
}

func autoHellbanCheck(authUser *database.User, lowerCaseMessage string) bool {
	checks := []string{
		"new here",
		"new to this",
		"new at this",
		"legit market",
		"help me",
	}
	if authUser.GeneralMessagesCount <= 5 {
		for _, check := range checks {
			if strings.Contains(lowerCaseMessage, check) {
				return true
			}
		}
	}
	return false
}
