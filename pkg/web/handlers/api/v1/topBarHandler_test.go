package v1

import (
	"dkforest/pkg/database"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

func TestColorifyTaggedUsers(t *testing.T) {
	getUsersByUsername := func(usernames []string) ([]database.User, error) {
		out := []database.User{
			{ID: 1, Username: "username1", ChatColor: "#001"},
			{ID: 2, Username: "username2", ChatColor: "#002"},
		}
		return out, nil
	}
	msg := "@username1 @username1 @username2 @username3"
	html, taggedUsersIDsMap := colorifyTaggedUsers(msg, getUsersByUsername)
	expected := `` +
		`<span style="color: #001; font-weight: normal; font-style: normal; font-family: Arial,Helvetica,sans-serif;">@username1</span> ` +
		`<span style="color: #001; font-weight: normal; font-style: normal; font-family: Arial,Helvetica,sans-serif;">@username1</span> ` +
		`<span style="color: #002; font-weight: normal; font-style: normal; font-family: Arial,Helvetica,sans-serif;">@username2</span> ` +
		`@username3`
	assert.Equal(t, expected, html)
	assert.Equal(t, 2, len(taggedUsersIDsMap))
	assert.Equal(t, "username1", taggedUsersIDsMap[1].Username)
	assert.Equal(t, "username2", taggedUsersIDsMap[2].Username)
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
		_, _ = colorifyTaggedUsers(msg, getUsersByUsername)
	}
}

func TestConvertMarkdown(t *testing.T) {
	// Convert markdown will not remove dangerous html
	msg := `<noscript>`
	out := convertMarkdown(msg)
	expected := "<p><noscript></p>\n"
	assert.Equal(t, expected, out)

	// Testing censored feature
	msg = `This #is censored# text`
	out = convertMarkdown(msg)
	expected = "<p>This <span class=\"censored\">is censored</span> text</p>\n"
	assert.Equal(t, expected, out)
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
