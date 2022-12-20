package v1

import (
	"dkforest/pkg/config"
	"dkforest/pkg/database"
	dutils "dkforest/pkg/database/utils"
	"dkforest/pkg/utils"
	"errors"
	"github.com/sirupsen/logrus"
	"strings"
	"time"
)

type SpamInterceptor struct{}

func (i SpamInterceptor) InterceptMsg(c *Command) {
	if err := checkSpam(c.origMessage, c.authUser); err != nil {
		c.err = err
		return
	}

	// Check CP links
	if checkCPLinks(c.message) {
		c.err = errors.New("forbidden url")
		return
	}
}

var ErrSpamFilterTriggered = errors.New("spam filter triggered")

func checkSpam(origMessage string, authUser *database.User) error {
	lowerCaseMessage := strings.ToLower(origMessage)
	silentSelfKick := config.SilentSelfKick.Load()

	// Kick retard new users
	if time.Since(authUser.CreatedAt) < 5*time.Hour {
		if strings.Contains(lowerCaseMessage, "fucked up links") ||
			strings.Contains(lowerCaseMessage, "i wanna see gore") ||
			strings.Contains(lowerCaseMessage, "how can i make money") ||
			strings.Contains(lowerCaseMessage, "any links for scary stuff") {
			dutils.SelfKick(*authUser, silentSelfKick)
			return ErrSpamFilterTriggered
		}
	}
	if authUser.GeneralMessagesCount < 20 || time.Since(authUser.CreatedAt) < 5*time.Hour {
		if strings.Contains(lowerCaseMessage, "cp link") {
			dutils.SelfKick(*authUser, silentSelfKick)
			return ErrSpamFilterTriggered
		}
	}

	if strings.Contains(lowerCaseMessage, "#dorkforest") {
		if authUser.IsModerator() {
			return ErrSpamFilterTriggered
		}
		dutils.SelfKick(*authUser, silentSelfKick)
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
			logrus.Error("failed unique ratio: " + origMessage)
			return errors.New("failed unique ratio")
		}
		if retardRatio > 0.1 {
			logrus.Error("failed retard ratio: " + origMessage)
			return errors.New("failed retard ratio")
		}
	}

	prophanityWords := []string{"cock", "dick", "nigger", "niggers", "nigga", "niggas", "sex"}
	prophanity := 0
	for _, w := range prophanityWords {
		if n, ok := wordsMap[w]; ok {
			prophanity += n
		}
	}

	if authUser.GeneralMessagesCount < 20 || time.Since(authUser.CreatedAt) < 5*time.Hour {
		if wordsMap["cp"] > 0 && (wordsMap["link"] > 0 || wordsMap["links"] > 0) {
			dutils.SelfKick(*authUser, silentSelfKick)
			return ErrSpamFilterTriggered
		}
	}

	return nil
}
