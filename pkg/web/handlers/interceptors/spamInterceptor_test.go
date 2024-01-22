package interceptors

import (
	"dkforest/pkg/database"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_autoHellbanCheck(t *testing.T) {
	type args struct {
		authUser         *database.User
		lowerCaseMessage string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{name: "", args: args{authUser: &database.User{GeneralMessagesCount: 2}, lowerCaseMessage: "hi new here"}, want: true},
		{name: "", args: args{authUser: &database.User{GeneralMessagesCount: 2}, lowerCaseMessage: "hello anybody know of any legit market places ? its getting tough on here to find any that actually do what they supposed to "}, want: true},
		{name: "", args: args{authUser: &database.User{GeneralMessagesCount: 2}, lowerCaseMessage: "Hello Guys and Ladys someone can help me? I Have a Little problem.."}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, autoHellbanCheck(tt.args.authUser, tt.args.lowerCaseMessage), "autoHellbanCheck(%v, %v)", tt.args.authUser, tt.args.lowerCaseMessage)
		})
	}
}

func Test_autoKickSpammers(t *testing.T) {
	type args struct {
		authUser         *database.User
		lowerCaseMessage string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{name: "", args: args{authUser: &database.User{GeneralMessagesCount: 2}, lowerCaseMessage: "blablabla l e m y _ b e a u t y on "}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, autoKickSpammers(tt.args.authUser, tt.args.lowerCaseMessage), "autoKickSpammers(%v, %v)", tt.args.authUser, tt.args.lowerCaseMessage)
		})
	}
}

func Test_autoKickProfanityTmp(t *testing.T) {
	type args struct {
		orig string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{"", args{orig: "biden is dumb fuck can suck my dick stupid nigger"}, true},
		{"", args{orig: "u can suck his nuts like the submissive faggot u are. slurping eye contact for deep man love with dirty butthole sniffing."}, true},
		{"", args{orig: "how to tear a human slut bitch from the cunt to the part in her hairline then shit into the chest cavity for happy dumpling poop soup"}, true},
		{"", args{orig: "lets murder a nun and fuck the blood scabs into her corpse pussy hole"}, true},
		{"", args{orig: "quick question, whats the best method to plant a grenaed in old ladys stinky rotted cunt hole to blast her bloods on a hotel walls"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, autoKickProfanityTmp(tt.args.orig), "autoKickProfanityTmp(%v)", tt.args.orig)
		})
	}
}
