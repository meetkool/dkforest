package interceptors

import (
	"dkforest/pkg/database"
	"dkforest/pkg/managers"
	"dkforest/pkg/web/handlers/interceptors/command"
	"strings"
)

type SnippetInterceptor struct{}

func (i SnippetInterceptor) InterceptMsg(cmd *command.Command) {
	// Snippets actually mutate the original message,
	// to simulate that the user actually typed the text
	cmd.OrigMessage = snippets(cmd.DB, cmd.AuthUser.ID, cmd.OrigMessage)

	cmd.OrigMessage = autocompleteTags(cmd.OrigMessage)

	cmd.Message = cmd.OrigMessage
}

func snippets(db *database.DkfDB, authUserID database.UserID, html string) string {
	if snippetRgx.MatchString(html) {
		userSnippets, _ := db.GetUserSnippets(authUserID)
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
		if strings.HasPrefix(s, `\`) {
			return strings.TrimPrefix(s, `\`)
		}
		s1 := strings.TrimPrefix(s, "@")
		s1 = strings.TrimSuffix(s1, "*")
		s1 = strings.ToLower(s1)
		for _, au := range activeUsers {
			l := strings.ToLower(string(au.Username))
			if strings.HasPrefix(l, s1) {
				return au.Username.AtStr()
			}
		}
		return s
	})
	return html
}
