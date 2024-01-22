package utils

import (
	bf "dkforest/pkg/blackfriday/v2"
	"dkforest/pkg/clockwork"
	"dkforest/pkg/config"
	"dkforest/pkg/database"
	"dkforest/pkg/hashset"
	"dkforest/pkg/levenshtein"
	"dkforest/pkg/managers"
	"dkforest/pkg/utils"
	"fmt"
	"github.com/ProtonMail/go-crypto/openpgp/clearsign"
	"github.com/microcosm-cc/bluemonday"
	html2 "html"
	"math"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	agePrefix       = "-----BEGIN AGE ENCRYPTED FILE-----"
	ageSuffix       = "-----END AGE ENCRYPTED FILE-----"
	pgpPrefix       = "-----BEGIN PGP MESSAGE-----"
	pgpSuffix       = "-----END PGP MESSAGE-----"
	pgpPKeyPrefix   = "-----BEGIN PGP PUBLIC KEY BLOCK-----"
	pgpPKeySuffix   = "-----END PGP PUBLIC KEY BLOCK-----"
	pgpSignedPrefix = "-----BEGIN PGP SIGNED MESSAGE-----"
	pgpSignedSuffix = "-----END PGP SIGNATURE-----"
)

var emojiReplacer = strings.NewReplacer(
	":):", `<span class="emoji" title=":):">☺</span>`,
	":smile:", `<span class="emoji" title=":smile:">☺</span>`,
	":happy:", `<span class="emoji" title=":happy:">😃</span>`,
	":see-no-evil:", `<span class="emoji" title=":see-no-evil:">🙈</span>`,
	":hear-no-evil:", `<span class="emoji" title=":hear-no-evil:">🙉</span>`,
	":speak-no-evil:", `<span class="emoji" title=":speak-no-evil:">🙊</span>`,
	":poop:", `<span class="emoji" title=":poop:">💩</span>`,
	":+1:", `<span class="emoji" title=":+1:">👍</span>`,
	":evil:", `<span class="emoji" title=":evil:">😈</span>`,
	":cat-happy:", `<span class="emoji" title=":cat-happy:">😸</span>`,
	":eyes:", `<span class="emoji" title=":eyes:">👀</span>`,
	":wave:", `<span class="emoji" title=":wave:">👋</span>`,
	":clap:", `<span class="emoji" title=":clap:">👏</span>`,
	":fire:", `<span class="emoji" title=":fire:">🔥</span>`,
	":sparkles:", `<span class="emoji" title=":sparkles:">✨</span>`,
	":sweat:", `<span class="emoji" title=":sweat:">💦</span>`,
	":heart:", `<span class="emoji" title=":heart:">❤</span>`,
	":broken-heart:", `<span class="emoji" title=":broken-heart:">💔</span>`,
	":anatomical-heart:", `<span class="emoji" title=":anatomical-heart:">🫀</span>`,
	":zzz:", `<span class="emoji" title=":zzz:">💤</span>`,
	":praise:", `<span class="emoji" title=":praise:">🙌</span>`,
	":joy:", `<span class="emoji" title=":joy:">😂</span>`,
	":sob:", `<span class="emoji" title=":sob:">😭</span>`,
	":pleading-face:", `<span class="emoji" title=":pleading-face:">🥺</span>`,
	":shush:", `<span class="emoji" title=":shush:">🤫</span>`,
	":scream:", `<span class="emoji" title=":scream:">😱</span>`,
	":heart-eyes:", `<span class="emoji" title=":heart-eyes:">😍</span>`,
	":blush:", `<span class="emoji" title=":blush:">☺</span>`,
	":crazy:", `<span class="emoji" title=":crazy:">😜</span>`,
	":angry:", `<span class="emoji" title=":angry:">😡</span>`,
	":triumph:", `<span class="emoji" title=":triumph:">😤</span>`,
	":vomit:", `<span class="emoji" title=":vomit:">🤮</span>`,
	":skull:", `<span class="emoji" title=":skull:">💀</span>`,
	":alien:", `<span class="emoji" title=":alien:">👽</span>`,
	":sleeping:", `<span class="emoji" title=":sleeping:">😴</span>`,
	":tongue:", `<span class="emoji" title=":tongue:">😛</span>`,
	":cool:", `<span class="emoji" title=":cool:">😎</span>`,
	":wink:", `<span class="emoji" title=":wink:">😉</span>`,
	":thinking:", `<span class="emoji" title=":thinking:">🤔</span>`,
	":happy-sweat:", `<span class="emoji" title=":happy-sweat:">😅</span>`,
	":nerd:", `<span class="emoji" title=":nerd:">🤓</span>`,
	":money-mouth:", `<span class="emoji" title=":money-mouth:">🤑</span>`,
	":fox:", `<span class="emoji" title=":fox:">🦊</span>`,
	":popcorn:", `<span class="emoji" title=":popcorn:">🍿</span>`,
	":money-bag:", `<span class="emoji" title=":money-bag:">💰</span>`,
	":facepalm:", `<span class="emoji" title=":facepalm:">🤦</span>`,
	":lungs:", `<span class="emoji" title=":lungs:">🫁</span>`,
	":shrug:", `¯\_(ツ)_/¯`,
	":flip:", `(╯°□°)╯︵ ┻━┻`,
	":flip-all:", `┻━┻︵ \(°□°)/ ︵ ┻━┻`,
	":fix-table:", `(ヘ･_･)ヘ┳━┳`,
	":disap:", `ಠ_ಠ`,
)

var usernameF = `\w{3,20}` // username (regex Fragment)
var roomNameF = `\w{3,50}`
var userOr0 = usernameF + `|0`
var optAtGUserOr0 = `@?(` + userOr0 + `)` // Optional @, Grouped, Username or 0
var pmRgx = regexp.MustCompile(`^/pm ` + optAtGUserOr0 + `(?:\s(?s:(.*)))?`)
var tagRgx = regexp.MustCompile(`(?:\\?)@(` + userOr0 + `)`)
var roomTagRgx = regexp.MustCompile(`#(` + roomNameF + `)`)
var noSchemeOnionLinkRgx = regexp.MustCompile(`\s[a-z2-7]{56}\.onion`)

var msgPolicy = bluemonday.NewPolicy().
	AllowElements("a", "p", "span", "strong", "del", "code", "pre", "em", "ul", "li", "br", "small", "i").
	AllowAttrs("href", "rel", "target").OnElements("a").
	AllowAttrs("tabindex", "style").OnElements("pre").
	AllowAttrs("style", "class", "title").OnElements("span").
	AllowAttrs("style").OnElements("small")

// ProcessRawMessage return the new html, and a map of tagged users used for notifications
// This function takes an "unsafe" user input "in", and return html which will be safe to render.
func ProcessRawMessage(db *database.DkfDB, in, roomKey string, authUserID database.UserID, roomID database.RoomID,
	upload *database.Upload, isModerator, canUseMultiline, manualML bool) (string, map[database.UserID]database.User, error) {
	html, quoted := convertQuote(db, in, roomKey, roomID, authUserID, isModerator) // Get raw quote text which is not safe to render
	html = convertNewLines(html, canUseMultiline)
	html = html2.EscapeString(html) // Makes user input safe to render
	// All html generated from this point on shall be safe to render.
	html = convertPGPClearsignToFile(db, html, authUserID)
	html = convertPGPMessageToFile(db, html, authUserID)
	html = convertPGPPublicKeyToFile(db, html, authUserID)
	html = convertAgeMessageToFile(db, html, authUserID)
	html = convertLinksWithoutScheme(html)
	html = convertMarkdown(db, html, canUseMultiline, manualML)
	html = convertBangShortcuts(html)
	html = convertArchiveLinks(db, html, roomID, authUserID)
	html = convertLinks(html, roomID, db.GetUserByUsername, db.GetLinkByShorthand, db.GetChatMessageByUUID)
	html = linkDefaultRooms(html)
	html, taggedUsersIDsMap := ColorifyTaggedUsers(html, db.GetUsersByUsername)
	html = emojiReplacer.Replace(html)
	html = styleQuote(html, quoted)
	html = appendUploadLink(html, upload)
	if quoted != nil { // Add quoted message owner for inboxes
		taggedUsersIDsMap[quoted.UserID] = quoted.User
	}
	html = msgPolicy.Sanitize(html)
	return html, taggedUsersIDsMap, nil
}

// This function will get the raw user input message which is not safe to directly render.
//
// To prevent people from altering the text of the quote,
// we retrieve the original quoted message using the timestamp and username,
// and we use the original message text.
//
// eg: we received altered quote, and return original quote ->
// “[01:23:45] username - Some maliciously altered quote” Some text
// “[01:23:45] username - The original text” Some text
func convertQuote(db *database.DkfDB, origHtml, roomKey string, roomID database.RoomID, authUserID database.UserID, isModerator bool) (html string, quoted *database.ChatMessage) {
	const quotePrefix = `“[`
	const quoteSuffix = `”`
	html = origHtml
	idx := strings.Index(origHtml, quoteSuffix)
	if strings.HasPrefix(origHtml, quotePrefix) && idx > -1 {
		prefixLen := len(quotePrefix)
		suffixLen := len(quoteSuffix)
		if len(origHtml) > prefixLen+9 {
			hourMinSec := origHtml[prefixLen : prefixLen+8]
			usernameStartIdx := prefixLen + 10
			spaceIdx := strings.Index(origHtml[usernameStartIdx:], " ")
			var username database.Username
			if spaceIdx >= 3 {
				username = database.Username(origHtml[usernameStartIdx : spaceIdx+usernameStartIdx])
			}
			if quoted = getQuotedChatMessage(db, hourMinSec, username, roomID, authUserID, isModerator); quoted != nil {
				html = GetQuoteTxt(db, roomKey, *quoted)
				html += origHtml[idx+suffixLen:]
			}
		}
	}
	return html, quoted
}

// Given a roomID and hourMinSec (01:23:45) and a username, retrieve the message from database that fits the predicates.
func getQuotedChatMessage(db *database.DkfDB, hourMinSec string, username database.Username, roomID database.RoomID, authUserID database.UserID, isModerator bool) (quoted *database.ChatMessage) {
	if dt, err := utils.ParsePrevDatetimeAt(hourMinSec, clockwork.NewRealClock()); err == nil {
		if msgs, err := db.GetRoomChatMessagesByDate(roomID, dt.UTC()); err == nil && len(msgs) > 0 {
			msg := msgs[0]
			if len(msgs) > 1 {
				for _, msgTmp := range msgs {
					if msgTmp.User.Username == username {
						msg = msgTmp
						break
					}
				}
			}
			if VerifyMsgAuth(db, &msg, authUserID, isModerator) {
				quoted = &msg
			}
		}
	}
	return
}

// GetQuoteTxt given a chat message, return the text to be used as a quote.
func GetQuoteTxt(db *database.DkfDB, roomKey string, quoted database.ChatMessage) (out string) {
	var err error
	decrypted, err := quoted.GetRawMessage(roomKey)
	if err != nil {
		return
	}
	if quoted.IsPm() {
		if m := pmRgx.FindStringSubmatch(decrypted); len(m) == 3 {
			decrypted = m[2]
		}
	} else if quoted.Moderators {
		decrypted = strings.TrimPrefix(decrypted, "/m ")
	} else if quoted.IsHellbanned {
		decrypted = strings.TrimPrefix(decrypted, "/hbm ")
	}
	isMe := false
	if strings.HasPrefix(decrypted, "/me") {
		isMe = true
		decrypted = strings.TrimPrefix(decrypted, "/me ")
	}

	startIdx := 0
	if strings.HasPrefix(decrypted, `“[`) {
		startIdx = strings.Index(decrypted, `” `)
		if startIdx == -1 {
			startIdx = 0
		} else {
			startIdx += len(`” `)
		}
	}

	decrypted = replTextPrefixSuffix(decrypted, agePrefix, ageSuffix, "[age.txt]")
	decrypted = replTextPrefixSuffix(decrypted, pgpPrefix, pgpSuffix, "[pgp.txt]")
	decrypted = replTextPrefixSuffix(decrypted, pgpPKeyPrefix, pgpPKeySuffix, "[pgp_pkey.txt]")

	remaining := " "
	if !quoted.System {
		remaining += fmt.Sprintf(`%s `, quoted.User.Username)
	}
	if quoted.UploadID != nil {
		if upload, err := db.GetUploadByID(*quoted.UploadID); err == nil {
			if decrypted != "" {
				decrypted += " "
			}
			decrypted += `[` + upload.OrigFileName + `]`
		}
	}
	if !isMe {
		remaining += "- "
	}

	toBeQuoted := decrypted[startIdx:]
	toBeQuoted = strings.ReplaceAll(toBeQuoted, "\n", ` `)
	toBeQuoted = strings.ReplaceAll(toBeQuoted, `“`, `"`)
	toBeQuoted = strings.ReplaceAll(toBeQuoted, `”`, `"`)

	remaining += utils.TruncStr2(toBeQuoted, 70, "…")
	return `“[` + quoted.CreatedAt.Format("15:04:05") + "]" + remaining + `”`
}

func convertNewLines(html string, canUseMultiline bool) string {
	if !canUseMultiline {
		html = strings.ReplaceAll(html, "\n", "")
	}
	return html
}

func ExtractPGPMessage(html string) (out string, start, end int) {
	pgpPrefixL := pgpPrefix
	pgpSuffixL := pgpSuffix
	startIdx := strings.Index(html, pgpPrefixL)
	endIdx := strings.Index(html, pgpSuffixL)
	if startIdx != -1 && endIdx != -1 {
		endIdx += len(pgpSuffixL)
		out = html[startIdx:endIdx]
		out = strings.TrimSpace(out)
		out = strings.TrimPrefix(out, pgpPrefixL)
		out = strings.TrimSuffix(out, pgpSuffixL)
		out = strings.Join(strings.Split(out, " "), "\n")
		out = pgpPrefixL + out
		out += pgpSuffixL
	}
	return out, startIdx, endIdx
}

func ExtractAgeMessage(html string) (out string, start, end int) {
	agePrefixL := agePrefix
	ageSuffixL := ageSuffix
	startIdx := strings.Index(html, agePrefixL)
	endIdx := strings.Index(html, ageSuffixL)
	if startIdx != -1 && endIdx != -1 {
		endIdx += len(ageSuffixL)
		out = html[startIdx:endIdx]
		out = strings.TrimSpace(out)
		out = strings.TrimPrefix(out, agePrefixL)
		out = strings.TrimSuffix(out, ageSuffixL)
		out = strings.Join(strings.Split(out, " "), "\n")
		out = agePrefixL + out
		out += ageSuffixL
	}
	return out, startIdx, endIdx
}

func extractPGPPublicKey(html string) (out string, start, end int) {
	pgpPKeyPrefixL := pgpPKeyPrefix
	pgpPKeySuffixL := pgpPKeySuffix
	startIdx := strings.Index(html, pgpPKeyPrefixL)
	endIdx := strings.Index(html, pgpPKeySuffixL)
	if startIdx != -1 && endIdx != -1 {
		endIdx += len(pgpPKeySuffixL)
		pkeySubSlice := html[startIdx:endIdx]
		unescapedPkey := html2.UnescapeString(pkeySubSlice)
		out = convertInlinePGPPublicKey(unescapedPkey)
	}
	return out, startIdx, endIdx
}

func extractPGPClearsign(html string) (out string, startIdx, endIdx int) {
	if b, _ := clearsign.Decode([]byte(html)); b != nil {
		pgpSignedPrefixL := pgpSignedPrefix
		pgpSignedSuffixL := pgpSignedSuffix
		startIdx = strings.Index(html, pgpSignedPrefixL)
		endIdx = strings.Index(html, pgpSignedSuffixL)
		endIdx += len(pgpSignedSuffixL)
		out = html[startIdx:endIdx]
	}
	return
}

func uploadAndHTML(db *database.DkfDB, authUserID database.UserID, html, fileName, content string, startIdx, endIdx int) string {
	upload, _ := db.CreateUpload(fileName, []byte(content), authUserID)
	msgBefore := html[0:startIdx]
	msgAfter := html[endIdx:]
	html = msgBefore + ` [` + upload.GetHTMLLink() + `] ` + msgAfter
	html = strings.TrimSpace(html)
	return html
}

// Auto convert pasted pgp public key into uploaded file
func convertPGPPublicKeyToFile(db *database.DkfDB, html string, authUserID database.UserID) string {
	if extracted, startIdx, endIdx := extractPGPPublicKey(html); extracted != "" {
		html = uploadAndHTML(db, authUserID, html, "pgp_pkey.txt", extracted, startIdx, endIdx)
	}
	return html
}

func convertPGPClearsignToFile(db *database.DkfDB, html string, authUserID database.UserID) string {
	if extracted, startIdx, endIdx := extractPGPClearsign(html); extracted != "" {
		html = uploadAndHTML(db, authUserID, html, "pgp_clearsign.txt", extracted, startIdx, endIdx)
	}
	return html
}

// Auto convert pasted pgp message into uploaded file
func convertPGPMessageToFile(db *database.DkfDB, html string, authUserID database.UserID) string {
	if extracted, startIdx, endIdx := ExtractPGPMessage(html); extracted != "" {
		html = uploadAndHTML(db, authUserID, html, "pgp.txt", extracted, startIdx, endIdx)
	}
	return html
}

// Auto convert pasted age message into uploaded file
func convertAgeMessageToFile(db *database.DkfDB, html string, authUserID database.UserID) string {
	if extracted, startIdx, endIdx := ExtractAgeMessage(html); extracted != "" {
		html = uploadAndHTML(db, authUserID, html, "age.txt", extracted, startIdx, endIdx)
	}
	return html
}

func convertInlinePGPPublicKey(inlinePKey string) string {
	pgpPKeyPrefixL := pgpPKeyPrefix
	pgpPKeySuffixL := pgpPKeySuffix
	// If it contains new lines, it was probably pasted using multi-line text box
	if strings.Contains(inlinePKey, "\n") {
		return inlinePKey
	}
	inlinePKey = strings.TrimSpace(inlinePKey)
	inlinePKey = strings.TrimPrefix(inlinePKey, pgpPKeyPrefixL)
	inlinePKey = strings.TrimSuffix(inlinePKey, pgpPKeySuffixL)
	inlinePKey = strings.TrimSpace(inlinePKey)
	commentsParts := strings.Split(inlinePKey, "Comment: ")
	commentsParts, lastCommentPart := commentsParts[:len(commentsParts)-1], commentsParts[len(commentsParts)-1]
	newCommentsParts := make([]string, 0)
	for idx := range commentsParts {
		if commentsParts[idx] != "" {
			commentsParts[idx] = "Comment: " + commentsParts[idx]
			commentsParts[idx] = strings.TrimSpace(commentsParts[idx])
			newCommentsParts = append(newCommentsParts, commentsParts[idx])
		}
	}

	rgx := regexp.MustCompile(`\s\s(\w|\+|/){64}`)
	m := rgx.FindStringIndex(lastCommentPart)
	commentsStr := ""
	key := ""
	if len(m) == 2 {
		idx := m[0]
		lastCommentP1 := lastCommentPart[:idx]
		lastCommentP2 := lastCommentPart[idx+2:]
		key = strings.Join(strings.Split(lastCommentP2, " "), "\n")
		commentsStr = strings.Join(newCommentsParts, "\n")
		commentsStr += "\nComment: " + lastCommentP1 + "\n\n"
	} else {
		key = "\n" + strings.Join(strings.Split(lastCommentPart, " "), "\n")
	}
	inlinePKey = pgpPKeyPrefixL + "\n" + commentsStr + key + "\n" + pgpPKeySuffixL
	return inlinePKey
}

// Fix up onion links that are missing the http scheme. This often happen when copy/pasting a link.
func convertLinksWithoutScheme(in string) string {
	html := noSchemeOnionLinkRgx.ReplaceAllStringFunc(in, func(s string) string {
		return " http://" + strings.TrimSpace(s)
	})
	return html
}

var linkRgxStr = `(http|ftp|https):\/\/([\w\-_]+(?:(?:\.[\w\-_]+)+))([\w\-\.,@?^=%&amp;:/~\+#\(\)]*[\w\-\@?^=%&amp;/~\+#\(\)])?`
var profileRgxStr = `/u/\w{3,20}`
var linkShorthandRgxStr = `/l/\w{3,20}`
var dkfArchiveRgx = regexp.MustCompile(`/chat/([\w_]{3,50})/archive\?uuid=([\w-]{36})#[\w-]{36}`)
var linkOrProfileRgx = regexp.MustCompile(`(` + linkRgxStr + `|` + profileRgxStr + `|` + linkShorthandRgxStr + `)`)
var userProfileLinkRgx = regexp.MustCompile(`^` + profileRgxStr + `$`)
var linkShorthandPageLinkRgx = regexp.MustCompile(`^` + linkShorthandRgxStr + `$`)
var youtubeComIDRgx = regexp.MustCompile(`watch\?v=([\w-]+)`)
var youtubeComShortsIDRgx = regexp.MustCompile(`/shorts/([\w-]+)`)
var youtuBeIDRgx = regexp.MustCompile(`https://youtu\.be/([\w-]+)`)
var yewtubeBeIDRgx = youtubeComIDRgx
var invidiousIDRgx = youtubeComIDRgx

func makeHtmlLink(label, link string) string {
	// We replace @ to prevent ColorifyTaggedUsers from trying to generate html inside the links.
	r := strings.NewReplacer("@", "&#64;", "#", "&#35;")
	label = r.Replace(label)
	link = r.Replace(link)
	return fmt.Sprintf(`<a href="%s" rel="noopener noreferrer" target="_blank">%s</a>`, link, label)
}

func splitQuote(in string) (string, string) {
	const quotePrefix = `<p>“[`
	const quoteSuffix = `”`
	idx := strings.Index(in, quoteSuffix)
	if idx == -1 || !strings.HasPrefix(in, quotePrefix) {
		return "", in
	}
	return in[:idx], in[idx:]
}

var LibredditURLs = []string{
	"http://spjmllawtheisznfs7uryhxumin26ssv2draj7oope3ok3wuhy43eoyd.onion",
	"http://fwhhsbrbltmrct5hshrnqlqygqvcgmnek3cnka55zj4y7nuus5muwyyd.onion",
	"http://kphht2jcflojtqte4b4kyx7p2ahagv4debjj32nre67dxz7y57seqwyd.onion",
	"http://inytumdgnri7xsqtvpntjevaelxtgbjqkuqhtf6txxhwbll2fwqtakqd.onion",
	"http://liredejj74h5xjqr2dylnl5howb2bpikfowqoveub55ru27x43357iid.onion",
	"http://kzhfp3nvb4qp575vy23ccbrgfocezjtl5dx66uthgrhu7nscu6rcwjyd.onion",
	"http://ecue64ybzvn6vjzl37kcsnwt4ycmbsyf74nbttyg7rkc3t3qwnj7mcyd.onion",
	"http://ledditqo2mxfvlgobxnlhrkq4dh34jss6evfkdkb2thlvy6dn4f4gpyd.onion",
	"http://ol5begilptoou34emq2sshf3may3hlblvipdjtybbovpb7c7zodxmtqd.onion",
	"http://lbrdtjaj7567ptdd4rv74lv27qhxfkraabnyphgcvptl64ijx2tijwid.onion",
}

var InvidiousURLs = []string{
	"http://c7hqkpkpemu6e7emz5b4vyz7idjgdvgaaa3dyimmeojqbgpea3xqjoid.onion",
	"http://kbjggqkzv65ivcqj6bumvp337z6264huv5kpkwuv6gu5yjiskvan7fad.onion",
	"http://grwp24hodrefzvjjuccrkw3mjq4tzhaaq32amf33dzpmuxe7ilepcmad.onion"}

var WikilessURLs = []string{
	"http://c2pesewpalbi6lbfc5hf53q4g3ovnxe4s7tfa6k2aqkf7jd7a7dlz5ad.onion",
	"http://dj2tbh2nqfxyfmvq33cjmhuw7nb6am7thzd3zsjvizeqf374fixbrxyd.onion"}

var NitterURLs = []string{
	"http://nitraeju2mipeziu2wtcrqsxg7h62v5y4eqgwi75uprynkj74gevvuqd.onion"}

var RimgoURLs = []string{
	"http://be7udfhmnzqyt7cxysg6c4pbawarvaofjjywp35nhd5qamewdfxl6sid.onion"}

func convertLinks(in string,
	roomID database.RoomID,
	getUserByUsername func(database.Username) (database.User, error),
	getLinkByShorthand func(string) (database.Link, error),
	getChatMessageByUUID func(string) (database.ChatMessage, error)) string {
	quote, rest := splitQuote(in)

	knownOnions := [][]string{
		{"http://git.dkf.onion", config.DkfGitOnion},
		{"http://dkfgit.onion", config.DkfGit1Onion},
		{"http://dread.onion", config.DreadOnion},
		{"http://cryptbb.onion", config.CryptbbOnion},
		{"http://blkhat.onion", config.BhcOnion},
		{"http://dnmx.onion", config.DnmxOnion},
		{"http://whonix.onion", config.WhonixOnion},
	}

	newRest := linkOrProfileRgx.ReplaceAllStringFunc(rest, func(link string) string {
		// Convert all occurrences of "/u/username" to a link to user profile page if the user exists
		if userProfileLinkRgx.MatchString(link) {
			user, err := getUserByUsername(database.Username(strings.TrimPrefix(link, "/u/")))
			if err != nil {
				return link
			}
			href := "/u/" + string(user.Username)
			return makeHtmlLink(href, href)
		}

		// Convert all occurrences of "/l/shorthand" to a link to link page if the shorthand exists
		if linkShorthandPageLinkRgx.MatchString(link) {
			l, err := getLinkByShorthand(strings.TrimPrefix(link, "/l/"))
			if err != nil {
				return link
			}
			href := "/l/" + *l.Shorthand
			return makeHtmlLink(href, href)
		}

		// Handle reddit links
		if strings.HasPrefix(link, "https://www.reddit.com/") {
			old := strings.Replace(link, "https://www.reddit.com/", "https://old.reddit.com/", 1)
			libredditLink := "/external-link/libreddit/" + url.PathEscape(strings.TrimPrefix(link, "https://www.reddit.com/"))
			oldHtmlLink := makeHtmlLink("old", old)
			libredditHtmlLink := makeHtmlLink("libredditLink", libredditLink)
			htmlLink := makeHtmlLink(link, link)
			return htmlLink + ` (` + oldHtmlLink + ` | ` + libredditHtmlLink + `)`
		} else if strings.HasPrefix(link, "https://old.reddit.com/") {
			libredditLink := "/external-link/libreddit/" + url.PathEscape(strings.TrimPrefix(link, "https://old.reddit.com/"))
			libredditHtmlLink := makeHtmlLink("libredditLink", libredditLink)
			htmlLink := makeHtmlLink(link, link)
			return htmlLink + ` (` + libredditHtmlLink + `)`
		}
		for _, libredditURL := range LibredditURLs {
			if strings.HasPrefix(link, libredditURL) {
				newPrefix := strings.Replace(link, libredditURL, "http://reddit.onion", 1)
				old := strings.Replace(link, libredditURL, "https://old.reddit.com", 1)
				oldHtmlLink := makeHtmlLink("old", old)
				htmlLink := makeHtmlLink(newPrefix, link)
				return htmlLink + ` (` + oldHtmlLink + `)`
			}
		}

		// Append YouTube link to invidious link
		for _, invidiousURL := range InvidiousURLs {
			if strings.HasPrefix(link, invidiousURL) {
				if strings.Contains(link, ".onion/watch?v=") {
					newPrefix := strings.Replace(link, invidiousURL, "http://invidious.onion", 1)
					m := invidiousIDRgx.FindStringSubmatch(link)
					if len(m) == 2 {
						videoID := m[1]
						youtubeLink := "https://www.youtube.com/watch?v=" + videoID
						youtubeHtmlLink := makeHtmlLink("Youtube", youtubeLink)
						htmlLink := makeHtmlLink(newPrefix, link)
						return htmlLink + ` (` + youtubeHtmlLink + `)`
					}
				}
			}
		}
		// Unknown invidious links
		if strings.Contains(link, ".onion/watch?v=") {
			m := invidiousIDRgx.FindStringSubmatch(link)
			if len(m) == 2 {
				videoID := m[1]
				youtubeLink := "https://www.youtube.com/watch?v=" + videoID
				youtubeHtmlLink := makeHtmlLink("Youtube", youtubeLink)
				htmlLink := makeHtmlLink(link, link)
				return htmlLink + ` (` + youtubeHtmlLink + `)`
			}
		}

		// Append wikiless link to wikipedia link
		if strings.HasPrefix(link, "https://en.wikipedia.org/") {
			wikilessLink := "/external-link/nitter/" + url.PathEscape(strings.TrimPrefix(link, "https://en.wikipedia.org/"))
			wikilessHtmlLink := makeHtmlLink("Wikiless", wikilessLink)
			htmlLink := makeHtmlLink(link, link)
			return htmlLink + ` (` + wikilessHtmlLink + `)`
		}
		for _, wikilessURL := range WikilessURLs {
			if strings.HasPrefix(link, wikilessURL) {
				newPrefix := strings.Replace(link, wikilessURL, "http://wikiless.onion", 1)
				wikipediaPrefix := strings.Replace(link, wikilessURL, "https://en.wikipedia.org", 1)
				wikipediaHtmlLink := makeHtmlLink("Wikipedia", wikipediaPrefix)
				htmlLink := makeHtmlLink(newPrefix, link)
				return htmlLink + ` (` + wikipediaHtmlLink + `)`
			}
		}

		// Append nitter link to twitter link
		if strings.HasPrefix(link, "https://twitter.com/") {
			nitterLink := "/external-link/nitter/" + url.PathEscape(strings.TrimPrefix(link, "https://twitter.com/"))
			nitterHtmlLink := makeHtmlLink("Nitter", nitterLink)
			htmlLink := makeHtmlLink(link, link)
			return htmlLink + ` (` + nitterHtmlLink + `)`
		}
		for _, nitterURL := range NitterURLs {
			if strings.HasPrefix(link, nitterURL) {
				newPrefix := strings.Replace(link, nitterURL, "http://nitter.onion", 1)
				twitterPrefix := strings.Replace(link, nitterURL, "https://twitter.com", 1)
				twitterHtmlLink := makeHtmlLink("Twitter", twitterPrefix)
				htmlLink := makeHtmlLink(newPrefix, link)
				return htmlLink + ` (` + twitterHtmlLink + `)`
			}
		}

		// Append rimgo link to imgur link
		if strings.HasPrefix(link, "https://imgur.com/") {
			rimgoLink := "/external-link/rimgo/" + url.PathEscape(strings.TrimPrefix(link, "https://imgur.com/"))
			rimgoHtmlLink := makeHtmlLink("Rimgo", rimgoLink)
			htmlLink := makeHtmlLink(link, link)
			return htmlLink + ` (` + rimgoHtmlLink + `)`
		}
		for _, rimgoURL := range RimgoURLs {
			if strings.HasPrefix(link, rimgoURL) {
				newPrefix := strings.Replace(link, rimgoURL, "http://rimgo.onion", 1)
				imgurPrefix := strings.Replace(link, rimgoURL, "https://imgur.com", 1)
				imgurHtmlLink := makeHtmlLink("Imgur", imgurPrefix)
				htmlLink := makeHtmlLink(newPrefix, link)
				return htmlLink + ` (` + imgurHtmlLink + `)`
			}
		}

		// Append invidious link to YouTube/yewtube link
		var videoID string
		var m []string
		var isShortUrl, isYewtube bool
		if strings.HasPrefix(link, "https://youtu.be/") {
			m = youtuBeIDRgx.FindStringSubmatch(link)
		} else if strings.HasPrefix(link, "https://www.youtube.com/watch?v=") {
			m = youtubeComIDRgx.FindStringSubmatch(link)
		} else if strings.HasPrefix(link, "https://yewtu.be/") || strings.HasPrefix(link, "https://www.yewtu.be/") {
			m = yewtubeBeIDRgx.FindStringSubmatch(link)
			isYewtube = true
		} else if strings.HasPrefix(link, "https://www.youtube.com/shorts/") {
			m = youtubeComShortsIDRgx.FindStringSubmatch(link)
			isShortUrl = true
		}
		if len(m) == 2 {
			videoID = m[1]
		}
		if videoID != "" {
			invidiousLink := "/external-link/invidious/" + url.PathEscape("watch?v="+videoID+"&local=true")
			invidiousHtmlLink := makeHtmlLink("Invidious", invidiousLink)
			htmlLink := makeHtmlLink(link, link)
			youtubeLink := "https://www.youtube.com/watch?v=" + videoID
			youtubeHtmlLink := makeHtmlLink("YT", youtubeLink)
			out := htmlLink + ` (` + invidiousHtmlLink + `)`
			if isShortUrl || isYewtube {
				out = htmlLink + ` (` + youtubeHtmlLink + ` | ` + invidiousHtmlLink + `)`
			}
			return out
		}

		// Special case for dkf links.
		{
			dkfLocalPrefix := "http://127.0.0.1:8080"
			dkfShortPrefix := "http://dkf.onion"
			dkfLongPrefix := config.DkfOnion
			hasLocalPrefix := strings.HasPrefix(link, dkfLocalPrefix)
			hasDkfShortPrefix := strings.HasPrefix(link, dkfShortPrefix)
			hasDkfLongPrefix := strings.HasPrefix(link, dkfLongPrefix)
			if hasLocalPrefix || hasDkfLongPrefix || hasDkfShortPrefix {
				var trimmed string
				if hasLocalPrefix {
					trimmed = strings.TrimPrefix(link, dkfLocalPrefix)
				} else if hasDkfLongPrefix {
					trimmed = strings.TrimPrefix(link, dkfLongPrefix)
				} else if hasDkfShortPrefix {
					trimmed = strings.TrimPrefix(link, dkfShortPrefix)
				}
				label := dkfShortPrefix + trimmed
				href := trimmed
				// Shorten archive links
				if m := dkfArchiveRgx.FindStringSubmatch(label); len(m) == 3 {
					if msg, err := getChatMessageByUUID(m[2]); err == nil {
						if roomID == msg.RoomID {
							label = msg.CreatedAt.Format("[Jan 02 03:04:05]")
						} else {
							label = msg.CreatedAt.Format("[#" + m[1] + " Jan 02 03:04:05]")
						}
					}
				}
				// Allows to have messages such as: "my profile is /u/username :)"
				if userProfileLinkRgx.MatchString(trimmed) {
					if user, err := getUserByUsername(database.Username(strings.TrimPrefix(trimmed, "/u/"))); err == nil {
						label = "/u/" + string(user.Username)
						href = "/u/" + string(user.Username)
					}
				} else if linkShorthandPageLinkRgx.MatchString(trimmed) {
					// Convert all occurrences of "/l/shorthand" to a link to link page if the shorthand exists
					if l, err := getLinkByShorthand(strings.TrimPrefix(trimmed, "/l/")); err == nil {
						label = "/l/" + *l.Shorthand
						href = "/l/" + *l.Shorthand
					}
				}
				return makeHtmlLink(label, href)
			}
		}

		for _, el := range knownOnions {
			shortPrefix := el[0]
			longPrefix := el[1]
			if strings.HasPrefix(link, longPrefix) {
				return makeHtmlLink(shortPrefix+strings.TrimPrefix(link, longPrefix), link)
			} else if strings.HasPrefix(link, shortPrefix) {
				return makeHtmlLink(link, longPrefix+strings.TrimPrefix(link, shortPrefix))
			}
		}
		return makeHtmlLink(link, link)
	})

	return quote + newRest
}

func appendUploadLink(html string, upload *database.Upload) string {
	if upload != nil {
		if html != "" {
			html += " "
		}
		html += `[` + upload.GetHTMLLink() + `]`
	}
	return html
}

type getUsersByUsernameFn func(usernames []string) ([]database.User, error)

// ColorifyTaggedUsers updates the given html to add user style for tags.
// Return the new html, and a map[userID]User of tagged users.
func ColorifyTaggedUsers(html string, getUsersByUsername getUsersByUsernameFn) (string, map[database.UserID]database.User) {
	tagRgxL := tagRgx
	usernameMatches := tagRgxL.FindAllStringSubmatch(html, -1)
	usernames := hashset.New[string]()
	for _, usernameMatch := range usernameMatches {
		if strings.HasPrefix(usernameMatch[0], `\`) {
			continue
		}
		usernames.Insert(usernameMatch[1])
	}
	taggedUsers, _ := getUsersByUsername(usernames.ToArray())

	taggedUsersMap := make(map[string]database.User)
	taggedUsersIDsMap := make(map[database.UserID]database.User)
	for _, taggedUser := range taggedUsers {
		taggedUsersMap[strings.ToLower(taggedUser.Username.AtStr())] = taggedUser
		if taggedUser.Username != config.NullUsername {
			taggedUsersIDsMap[taggedUser.ID] = taggedUser
		}
	}

	if len(usernameMatches) > 0 {
		html = tagRgxL.ReplaceAllStringFunc(html, func(s string) string {
			if strings.HasPrefix(s, `\`) {
				return strings.TrimPrefix(s, `\`)
			}
			lowerS := strings.ToLower(s)
			if user, ok := taggedUsersMap[lowerS]; ok {
				return fmt.Sprintf("<span %s>@%s</span>", user.GenerateChatStyle1(), user.Username)
			}

			// Not found, try to fix typos using levenshtein
			activeUsers := managers.ActiveUsers.GetActiveUsers()
			if len(activeUsers) > 0 {
				minDist := math.MaxInt
				minAu := activeUsers[0]
				for _, au := range activeUsers {
					lowerAu := strings.ToLower(string(au.Username))
					d := levenshtein.ComputeDistance(strings.TrimPrefix(lowerS, "@"), lowerAu)
					if d < minDist {
						minDist = d
						minAu = au
					}
				}
				if minDist <= 3 {
					if users, _ := getUsersByUsername([]string{minAu.Username.String()}); len(users) > 0 {
						user := users[0]
						return fmt.Sprintf("<span %s>@%s</span>", user.GenerateChatStyle1(), user.Username)
					}
				}
			}

			return s
		})
	}
	return html, taggedUsersIDsMap
}

func linkDefaultRooms(html string) string {
	r := strings.NewReplacer(
		"#general", `<a href="/chat/general" target="_top">#general</a>`,
		"#programming", `<a href="/chat/programming" target="_top">#programming</a>`,
		"#hacking", `<a href="/chat/hacking" target="_top">#hacking</a>`,
		"#suggestions", `<a href="/chat/suggestions" target="_top">#suggestions</a>`,
		"#announcements", `<a href="/chat/announcements" target="_top">#announcements</a>`,
	)
	return r.Replace(html)
}

// Convert timestamps such as 01:23:45 to an archive link if a message with that timestamp exists.
// eg: "Some text 14:31:46 some more text"
func convertArchiveLinks(db *database.DkfDB, html string, roomID database.RoomID, authUserID database.UserID) string {
	start, rest := "", html

	// Do not replace timestamps that are inside a quote text
	const quoteSuffix = `”`
	endOfQuoteIdx := strings.LastIndex(html, quoteSuffix)
	if endOfQuoteIdx != -1 {
		start, rest = html[:endOfQuoteIdx], html[endOfQuoteIdx:]
	}

	archiveRgx := regexp.MustCompile(`(\d{2}-\d{2} )?\d{2}:\d{2}:\d{2}`)
	if archiveRgx.MatchString(rest) {
		rest = archiveRgx.ReplaceAllStringFunc(rest, func(s string) string {
			var dt time.Time
			var err error
			if len(s) == 8 { // HH:MM:SS
				dt, err = utils.ParsePrevDatetimeAt(s, clockwork.NewRealClock())
			} else if len(s) == 14 { // mm-dd HH:MM:SS
				dt, err = utils.ParsePrevDatetimeAt2(s, clockwork.NewRealClock())
			}
			if err != nil {
				return s
			}
			if msgs, err := db.GetRoomChatMessagesByDate(roomID, dt.UTC()); err == nil && len(msgs) > 0 {
				msg := msgs[0]
				if len(msgs) > 1 {
					for _, msgTmp := range msgs {
						if msgTmp.User.ID == authUserID || (msgTmp.ToUserID != nil && *msgTmp.ToUserID == authUserID) {
							msg = msgTmp
							break
						}
					}
				}
				return fmt.Sprintf(`<a href="/chat/%s/archive#%s" target="_blank" rel="noopener noreferrer">%s</a>`, msg.Room.Name, msg.UUID, s)
			}
			return s
		})
	}
	return start + rest
}

func convertBangShortcuts(html string) string {
	r := strings.NewReplacer(
		"!bhc", config.BhcOnion,
		"!cryptbb", config.CryptbbOnion,
		"!dread", config.DreadOnion,
		"!dkf", config.DkfOnion,
		"!rroom", config.DkfOnion+`/red-room`,
		"!dnmx", config.DnmxOnion,
		"!whonix", config.WhonixOnion,
		"!age", config.AgeUrl,
	)
	return r.Replace(html)
}

func convertMarkdown(db *database.DkfDB, in string, canUseMultiline, manualML bool) string {
	out := strings.Replace(in, "\r", "", -1)
	flags := bf.NoIntraEmphasis | bf.Tables | bf.FencedCode | bf.Strikethrough | bf.SpaceHeadings |
		bf.DefinitionLists | bf.HardLineBreak | bf.NoLink
	if canUseMultiline && manualML {
		flags |= bf.ManualLineBreak
	}
	resBytes := bf.Run([]byte(out), bf.WithRenderer(database.MyRenderer(db, false, false)), bf.WithExtensions(flags))
	out = string(resBytes)
	return out
}

func styleQuote(origHtml string, quoted *database.ChatMessage) (html string) {
	const quoteSuffix = `”`
	html = origHtml
	if quoted != nil {
		idx := strings.Index(origHtml, quoteSuffix)
		prefixLen := len(`<p>“[`)
		suffixLen := len(quoteSuffix)
		dateLen := 8 // 01:23:45 --> 8

		// <p>“[01:23:45] username - quoted text” user text</p>
		date := origHtml[prefixLen : prefixLen+dateLen] // `01:23:45`
		quoteTxt := origHtml[prefixLen+dateLen+1 : idx] // ` username - quoted text`
		userTxt := origHtml[idx+suffixLen:]             // ` user text</p>`

		sb := strings.Builder{}
		sb.WriteString(`<p>“<small style="opacity: 0.8;"><i>[`)

		// Date link
		sb.WriteString(`<a href="/chat/`)
		sb.WriteString(quoted.Room.Name)
		sb.WriteString(`/archive#`)
		sb.WriteString(quoted.UUID)
		sb.WriteString(`" target="_blank" rel="noopener noreferrer">`)
		sb.WriteString(date)
		sb.WriteString(`</a>`)

		sb.WriteString(`]<span `)
		sb.WriteString(quoted.User.GenerateChatStyle1())
		sb.WriteString(`>`)
		sb.WriteString(quoteTxt)
		sb.WriteString(`</span></i></small>”`)
		sb.WriteString(userTxt)
		html = sb.String()
	}
	return html
}

func replTextPrefixSuffix(msg, prefix, suffix, repl string) (out string) {
	out = msg
	pgpPIdx := strings.Index(msg, prefix)
	pgpSIdx := strings.Index(msg, suffix)
	if pgpPIdx != -1 && pgpSIdx != -1 {
		newMsg := msg[:pgpPIdx]
		newMsg += repl
		newMsg += msg[pgpSIdx+len(suffix):]
		out = newMsg
	}
	return
}
