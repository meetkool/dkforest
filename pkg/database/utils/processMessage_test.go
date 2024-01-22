package utils

import (
	"dkforest/pkg/database"
	"errors"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
	"time"
)

func TestConvertMarkdown(t *testing.T) {
	// Convert markdown will not remove dangerous html
	msg := `<noscript>`
	out := convertMarkdown(msg, false, false)
	expected := "<p><noscript></p>\n"
	assert.Equal(t, expected, out)

	// Testing censored feature
	msg = `This #is censored# text`
	out = convertMarkdown(msg, false, false)
	expected = "<p>This <span class=\"censored\">is censored</span> text</p>\n"
	assert.Equal(t, expected, out)
}

func TestGetQuoteTxt(t *testing.T) {
	// Quotes do not include original message quote if any
	txt := GetQuoteTxt(nil, "", database.ChatMessage{RawMessage: `“[00:00:01] user1 - this is a test” another one`, User: database.User{Username: "user1"}, CreatedAt: time.Date(0, 0, 0, 0, 0, 2, 0, time.UTC)})
	expected := "“[00:00:02] user1 - another one”"
	assert.Equal(t, expected, txt)

	// Quoted should inline a multiline message
	txt = GetQuoteTxt(nil, "", database.ChatMessage{RawMessage: "line1\nline2\nline3", User: database.User{Username: "user1"}, CreatedAt: time.Date(0, 0, 0, 0, 0, 2, 0, time.UTC)})
	expected = "“[00:00:02] user1 - line1 line2 line3”"
	assert.Equal(t, expected, txt)

	// Quoted should replace "special double quotes" to normal "double quote"
	txt = GetQuoteTxt(nil, "", database.ChatMessage{RawMessage: `instead of showing
“[00:00:01] user2 - an article about HHVM...
https://www.zapbuild.com/bitsntricks/hiphop-…” that looks perfect for my chat
i think it would be better if it shows
“[00:00:01] user2 - an article about HHVM...” that looks perfect for my chat`, User: database.User{Username: "user1"}, CreatedAt: time.Date(0, 0, 0, 0, 0, 2, 0, time.UTC)})
	expected = "“[00:00:02] user1 - instead of showing \"[00:00:01] user2 - an article about HHVM...…”"
	assert.Equal(t, expected, txt)
}

func TestConvertPGPPublicKey(t *testing.T) {
	// Test public key with comment
	originPKey := `-----BEGIN PGP PUBLIC KEY BLOCK-----
Comment: User-ID:       SOME COMMENT
Comment: Fingerprint:   140A9010B54795FCF10F4DC43B2D5AEC63C059D5

mDMEYjY6NhYJKwYBBAHaRw8BAQdAf6Csr2KO/xS45wIATAE2ReVclrl54qP6+1GO
EsRtZTXRbbiR809TVRcFOn5iro5Ez6Q6K1d7fRwtfwEAz7cX1RUfG0r5cwM03/RG
1mQYZZz6Qapo73bGygjx7AI=
=XmRh
-----END PGP PUBLIC KEY BLOCK-----`
	inlinePKey := strings.Join(strings.Split(originPKey, "\n"), " ")
	converted := convertInlinePGPPublicKey(inlinePKey)
	assert.Equal(t, originPKey, converted)

	// Test public key without comment
	originPKey = `-----BEGIN PGP PUBLIC KEY BLOCK-----

mQINBGEeQ0kBEADUTVcGuD1sY4Lsn0Bep/T1XZeOeOdT0ZCMJr9Giksb4yhcNPIL
uhnpGDkYIH5ZJdLq1IiEAbctPcitmwNQcSiDS23iSdino8draQ1YrPLHiNx6RZk0
FoEVD2av5BES9MvnPsQulj9bU2lUokhBjM1+LERxbqfVfZ2ddAYRIMGF
=G+E+
-----END PGP PUBLIC KEY BLOCK-----`
	inlinePKey = strings.Join(strings.Split(originPKey, "\n"), " ")
	converted = convertInlinePGPPublicKey(inlinePKey)
	assert.Equal(t, originPKey, converted)

	originPKey = `-----BEGIN PGP PUBLIC KEY BLOCK-----
Comment: EMAIL:         <email@email.com>
Comment: Fingerprint:   140A9010B54795FCF10F4DC43B2D5AEC63C059D5

mDMEYjY6NhYJKwYBBAHaRw8BAQdAf6Csr2KO/xS45wIATAE2ReVclrl54qP6+1GO
EsRtZTXRbbiR809TVRcFOn5iro5Ez6Q6K1d7fRwtfwEAz7cX1RUfG0r5cwM03/RG
1mQYZZz6Qapo73bGygjx7AI=
=XmRh
-----END PGP PUBLIC KEY BLOCK-----`
	inlinePKey = strings.Join(strings.Split(originPKey, "\n"), " ")
	converted = convertInlinePGPPublicKey(inlinePKey)
	assert.Equal(t, originPKey, converted)
}

func TestConvertLinks(t *testing.T) {
	getUserByUsername := func(username database.Username) (database.User, error) {
		if strings.ToLower(string(username)) == "username" {
			return database.User{Username: "username"}, nil
		}
		return database.User{}, errors.New("not exists")
	}
	getLinkByShorthand := func(shorthand string) (database.Link, error) {
		return database.Link{}, nil
	}
	getChatMessageByUUID := func(uuid string) (database.ChatMessage, error) {
		return database.ChatMessage{}, nil
	}

	// Replace /u/username to link when user exists
	actual := convertLinks("this is /u/username a test", 0, getUserByUsername, getLinkByShorthand, getChatMessageByUUID)
	expected := `this is <a href="/u/username" rel="noopener noreferrer" target="_blank">/u/username</a> a test`
	assert.Equal(t, expected, actual)

	// Does not replace /u/notExist to link when user does not exist
	actual = convertLinks("this is /u/notExist a test", 0, getUserByUsername, getLinkByShorthand, getChatMessageByUUID)
	expected = `this is /u/notExist a test`
	assert.Equal(t, expected, actual)

	// Fix case errors
	actual = convertLinks("this is /u/uSerNaMe a test", 0, getUserByUsername, getLinkByShorthand, getChatMessageByUUID)
	expected = `this is <a href="/u/username" rel="noopener noreferrer" target="_blank">/u/username</a> a test`
	assert.Equal(t, expected, actual)

	// Convert long dkf url
	actual = convertLinks("this is http://dkforestseeaaq2dqz2uflmlsybvnq2irzn4ygyvu53oazyorednviid.onion/u/username a test", 0, getUserByUsername, getLinkByShorthand, getChatMessageByUUID)
	expected = `this is <a href="/u/username" rel="noopener noreferrer" target="_blank">/u/username</a> a test`
	assert.Equal(t, expected, actual)

	// Shorten dkf url but keep the short form since the user does not exist
	actual = convertLinks("this is http://dkforestseeaaq2dqz2uflmlsybvnq2irzn4ygyvu53oazyorednviid.onion/u/notExist a test", 0, getUserByUsername, getLinkByShorthand, getChatMessageByUUID)
	expected = `this is <a href="/u/notExist" rel="noopener noreferrer" target="_blank">http://dkf.onion/u/notExist</a> a test`
	assert.Equal(t, expected, actual)
}

func TestColorifyTaggedUsers(t *testing.T) {
	getUsersByUsername := func(usernames []string) ([]database.User, error) {
		out := []database.User{
			{ID: 1, Username: "username1", ChatColor: "#001"},
			{ID: 2, Username: "username2", ChatColor: "#002"},
		}
		return out, nil
	}
	msg := "@username1 @username1 @username2 @username3"
	html, taggedUsersIDsMap := ColorifyTaggedUsers(msg, getUsersByUsername)
	expected := `` +
		`<span style="color: #001; font-weight: normal; font-style: normal; font-family: Arial,Helvetica,sans-serif;">@username1</span> ` +
		`<span style="color: #001; font-weight: normal; font-style: normal; font-family: Arial,Helvetica,sans-serif;">@username1</span> ` +
		`<span style="color: #002; font-weight: normal; font-style: normal; font-family: Arial,Helvetica,sans-serif;">@username2</span> ` +
		`@username3`
	assert.Equal(t, expected, html)
	assert.Equal(t, 2, len(taggedUsersIDsMap))
	assert.Equal(t, database.Username("username1"), taggedUsersIDsMap[1].Username)
	assert.Equal(t, database.Username("username2"), taggedUsersIDsMap[2].Username)
}

func BenchmarkColorifyTaggedUsers(b *testing.B) {
	getUsersByUsername := func(usernames []string) ([]database.User, error) {
		out := []database.User{
			{ID: 1, Username: "username1", ChatColor: "#000"},
			{ID: 2, Username: "username2", ChatColor: "#001"},
		}
		return out, nil
	}
	msg := "@username1 @username1 @username2 @username3"
	for n := 0; n < b.N; n++ {
		_, _ = ColorifyTaggedUsers(msg, getUsersByUsername)
	}
}
