package v1

import (
	"dkforest/pkg/database"
	"dkforest/pkg/managers"
	"regexp"
	"strings"
)

type SnippetInterceptor struct{}

func (i SnippetInterceptor) InterceptMsg(cmd *Command) {
	// Snippets actually mutate the original message,
	// to simulate that the user actually typed the text
	cmd.origMessage = snippets(cmd.authUser.ID, cmd.origMessage)

	cmd.origMessage = autocompleteTags(cmd.origMessage)

	cmd.message = cmd.origMessage
}

var snippetRgx = regexp.MustCompile(`!\w{1,20}`)

func snippets(authUserID database.UserID, html string) string {
	if snippetRgx.MatchString(html) {
		userSnippets, _ := database.GetUserSnippets(authUserID)
		if len(userSnippets) > 0 {
			// Build hashmap for fast lookup
			m := make(map[string]string)
			for _, snippet := range userSnippets {
				m["!"+snippet.Name] = snippet.Text
			}
			html = snippetRgx.ReplaceAllStringFunc(html, func(s string) string {
				// If snippet name exists, use the mapped value
				if v, ok := m[s]; ok {
					return v
				}
				return s
			})
		}
	}
	return html
}

func autocompleteTags(html string) string {
	activeUsers := managers.ActiveUsers.GetActiveUsers()
	html = autoTagRgx.ReplaceAllStringFunc(html, func(s string) string {
		s1 := strings.TrimPrefix(s, "@")
		s1 = strings.TrimSuffix(s1, "*")
		s1 = strings.ToLower(s1)
		for _, au := range activeUsers {
			l := strings.ToLower(au.Username)
			if strings.HasPrefix(l, s1) {
				return "@" + au.Username
			}
		}
		return s
	})
	return html
}
