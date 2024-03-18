package v1

import (
	"fmt"
	"html/template"
	"io"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/valyala/quicktemplate"
)

type ChatMessagesData struct {
	RoomName          string
	TopBarQueryParams string
	NbButtons         int
	Members           []managers.UserInfo
	MembersInChat     []managers.UserInfo
	OfficialRooms     []managers.RoomInfo
	SubscribedRooms   []managers.RoomInfo
	InboxCount        int
	VisibleMemberInChat bool
	DisplayHellbanned bool
}

type ChatMessage struct {
	UUID        string
	Rev         int
	CreatedAt   time.Time
	User        managers.UserInfo
	Message     string
	System      bool
	Moderators  bool
	GroupID     *string
	ToUserID    *int
	IsHellbanned bool
}

func StreamGenerateStyle(w *quicktemplate.Writer, authUser *database.User, data ChatMessagesData) {
	w.N().S(`<style>
   /* http://meyerweb.com/eric/tools/css/reset/
       v2.0 | 20110126
       License: none (public domain)
    */
    html, body, div, span, applet, object, iframe,
    h1, h2, h3, h4, h5, h6, p, blockquote, pre,
    a, abbr, acronym, address, big, cite, code,
    del, dfn, em, img, ins, kbd, q, s, samp,
    small, strike, strong, sub, sup, tt, var,
    b, u, i, center,
    dl, dt, dd, ol, ul, li,
    fieldset, form, label, legend,
    table, caption, tbody, tfoot, thead, tr, th, td,
    article, aside, canvas, details, embed,
    figure, figcaption, footer, header, hgroup,
    menu, nav, output, ruby, section, summary,
    time, mark, audio, video {
        margin: 0;
        padding: 0;
        border: 0;
        font-size: 100%;
        font: inherit;
        vertical-align: baseline;
    }
    /* HTML5 display-role reset for older browsers */
    article, aside, details, figcaption, figure,
    footer, header, hgroup, menu, nav, section {
        display: block;
    }
    body {
        line-height: 1;
    }
    ol, ul {
        list-style: none;
    }
    blockquote, q {
        quotes: none;
    }
    blockquote:before, blockquote:after,
    q:before, q:after {
        content: '';
        content: none;
    }
    table {
        border-collapse: collapse;
        border-spacing: 0;
    }
    /* --- end --- */

    i { font-style: italic; }

    /* Remove button padding in FF */
    button::-moz-focus-inner {
        border:0;
        padding:0;
    }

    body { font-family: Lato,-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,"Helvetica Neue",Arial,sans-serif,"Apple Color Emoji","Segoe UI Emoji","Segoe UI Symbol"; }
    a       { color: #00bc8c; text-decoration: none; }
    a:hover { color: #007053; text-decoration: underline; }
    .unread_room       { color: #2392da; text-decoration: none; }
    .unread_room:hover { color: #004970; text-decoration: underline; }
    .emoji {
        font-family: "Noto Color Emoji", "Apple Color Emoji", "Segoe UI Emoji", Times, Symbola, Aegyptus, Code2000, Code2001, Code2002, Musica, serif, LastResort;
        font-size: 17px;
    }
    .mod-btn {
        width: 16px; height: 16px;
        margin: 0; padding: 0;
        border: 1px solid gray;
        display: inline;
        text-align: center;
        vertical-align: middle;
        user-select: none;
        background-color: #444;
        color: #ea2a2a;
        -webkit-box-shadow: 1px 1px 1px rgba(0,0,0,0.25);
        -moz-box-shadow: 1px 1px 1px rgba(0,0,0,0.25);
        -webkit-border-radius: 3px;
        -moz-border-radius: 3px;
    }
    .mod-btn:hover {
        background-color: #222;
    }
    .delete_msg_btn {
        font-size: 15px;
        line-height: 1;
    }
    @keyframes hide_btn {
        100% { visibility: hidden;  }
    }
    @keyframes orange_btn {
        99% { color: ea2a2a; } 100% { color: orange;  }
    }
    .delete_msg_btn::after { content: "×"; }
    .hb_btn {
        font-size: 10px;
        line-height: 1.4;
    }
    .hb_btn::after { content: "hb"; }
    .k_btn {
        font-size: 10px;
        line-height: 1.4;
    }
    .k_btn::after { content:
