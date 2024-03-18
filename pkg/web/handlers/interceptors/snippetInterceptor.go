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
	cmd.OrigMessage = snippets(cmd.DB, cmd.AuthUser.ID(), cmd.OrigMessage)
	cmd.OrigMessage = autocompleteTags(cmd.OrigMessage)
	cmd.Message = cmd.OrigMessage
}

func snippets(db *database.DkfDB, authUserID database.UserID, html string) string {
	if !snippetRgx.MatchString(html) {
		return html
	}

	userSnippets, err := db.GetUserSnippets(authUserID)
	if err != nil {
		return html
	}

	if len(userSnippets) == 0 {
		return html
	}

	// Build hashmap for fast lookup
	m := make(map[string]string)
	for _, snippet := range userSnippets {
		m["!"+snippet.Name] = snippet.Text
	}

	return snippetRgx.ReplaceAllStringFunc(html, func(s string) string {
		// If snippet name exists, use the mapped value
		if v, ok := m[s]; ok {
			return v
		}
		return s
	})
}

func autocompleteTags(html string) string {
	activeUsers := managers.ActiveUsers.GetActiveUsers()

	return autoTagRgx.ReplaceAllStringFunc(html, func(s string) string {
		if strings.HasPrefix(s, `\`) {
			return strings.TrimPrefix(s, `\`)
		}

		s1 := strings.TrimPrefix(s, "@")
		s1 = strings.TrimSuffix
