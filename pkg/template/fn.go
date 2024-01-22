package template

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"dkforest/pkg/database"
	"dkforest/pkg/utils"
	humanize "github.com/dustin/go-humanize"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

func getKeyFingerprint(pkey string) string {
	return utils.GetKeyFingerprint(pkey)
}

func shortNum(n int64) string {
	if n < 1000 {
		return utils.FormatInt64(n)
	} else if n >= 1000 && n < 1000000 {
		return utils.FormatInt64(n/1000) + "k"
	} else if n >= 1000000 {
		return utils.FormatInt64(n/1000000) + "M"
	}
	return utils.FormatInt64(n)
}

func shortNumPtr(n *int64) string {
	if n == nil {
		return "-"
	}
	return shortNum(*n)
}

func n(start, end int64) (stream chan int64) {
	stream = make(chan int64)
	utils.SGo(func() {
		for i := start; i <= end; i++ {
			stream <- i
		}
		close(stream)
	})
	return
}

func mod(i, j int64) bool { return i%j == 0 }

func addInt(a, b int) int {
	return a + b
}
func add(a, b int64) int64 {
	return a + b
}

func md5(v string) string {
	return utils.MD5([]byte(v))
}

func limitTo(limit int64, s string) string {
	if int64(len(s)) > limit {
		return s[0:limit]
	}
	return s
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func css(s string) template.CSS {
	return template.CSS(s)
}

func attr(s string) template.HTMLAttr {
	return template.HTMLAttr(s)
}

func safe(s string) template.HTML {
	return template.HTML(s)
}

func safeURL(s string) template.URL {
	return template.URL(s)
}

func safeJsStr(s string) template.JSStr {
	return template.JSStr(s)
}

func safeJs(s string) template.JS {
	return template.JS(s)
}

func success(code int64) bool {
	return code/100 == 2
}

func divide100(val int64) float64 {
	return float64(val) / 100
}

func divide1000(val int64) float64 {
	return float64(val) / 1000
}
func divide100M(val int64) float64 {
	return float64(val) / 100_000_000
}
func divide1T(val database.Piconero) float64 {
	return float64(val) / 1_000_000_000_000
}
func fmtPiconero(val database.Piconero) string {
	return val.XmrStr()
}

func int64bytes(val int64) string {
	return humanize.Bytes(uint64(val))
}

func toString(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func toI64(v any) int64 {
	if i, ok := v.(int64); ok {
		return i
	}
	return 0
}

func formatFloat(precision int, val float64) string {
	return strconv.FormatFloat(val, 'f', precision, 64)
}

func comma(v int64) string {
	return humanize.Comma(int64(v))
}

func intComma(v int) string {
	return humanize.Comma(int64(v))
}

func uint64Comma(v uint64) string {
	return humanize.Comma(int64(v))
}

func uint32Comma(v uint32) string {
	return humanize.Comma(int64(v))
}

func unixNs(v uint64) time.Time {
	return time.Unix(0, int64(v))
}

func commaPtr(v *int64) string {
	if v == nil {
		return "-"
	}
	return comma(*v)
}

func nowOGTFmt(location *time.Location) string {
	if location == nil {
		return "-- h --"
	}
	return time.Now().In(location).Format("15 h 04")
}

func formatTsPtr(v *time.Time) string {
	if v == nil {
		return "n/a"
	}
	return v.Format("Jan 02, 2006 - 15:04:05")
}

func formatLocal(date time.Time) string {
	return date.Local().Format("06.01.02 15:04:05")
}

func until(date time.Time) string {
	return humanize.Time(date)
}

func notNil(v any) any {
	if v == nil || (reflect.ValueOf(v).Kind() == reflect.Ptr && reflect.ValueOf(v).IsNil()) {
		return ""
	}
	return v
}

func shortDurNs(ns uint64) string {
	return time.Duration(ns).String()
}

func shortDur(v any) string {
	return utils.ShortDur(v)
}

func secs(t time.Time) int64 {
	return int64(time.Until(t).Seconds())
}

func backtoIP4(longIP uint32) string {
	return utils.BacktoIP4(int64(longIP))
}

func ts(ts int64) time.Time {
	return time.Unix(int64(ts), 0)
}

func capfirst(in string) string {
	if len(in) <= 0 {
		return ""
	}
	r, size := utf8.DecodeRuneInString(in)
	return strings.ToUpper(string(r)) + in[size:]
}

func first(in string) string {
	return string(in[0])
}

func last(in string) string {
	return string(in[len(in)-1])
}

func rest(in string) string {
	return in[1:]
}

func middle(in string) string {
	return in[1 : len(in)-1]
}

func translate(varName string, vals templateDataStruct) string {
	sections := []string{
		vals.TmplName + "." + varName,
	}
	parts := strings.Split(vals.TmplName, ".")
	for len(parts) > 0 {
		parts = parts[0 : len(parts)-1]
		sections = append(sections, strings.Join(append(parts, "index"), ".")+"."+varName)
	}
	var translated string
	var err error
	localizer := i18n.NewLocalizer(vals.Bundle, vals.Lang, vals.AcceptLanguage)
	for _, section := range sections {
		translated, err = localizer.Localize(&i18n.LocalizeConfig{
			DefaultMessage: &i18n.Message{ID: section},
			TemplateData:   vals.Data,
		})
		if err != nil {
			continue
		}
		break
	}
	if err != nil {
		translated = localizer.MustLocalize(&i18n.LocalizeConfig{
			DefaultMessage: &i18n.Message{ID: "global." + varName, Other: varName},
			TemplateData:   vals.Data,
		})
	}
	return translated
}

func trimPrefix(prefix, val string) string {
	return strings.TrimPrefix(val, prefix)
}

func mul(param1 int64, param2 int64) int64 {
	return param1 * param2
}

func trunc(nb int64, in string) string {
	if int64(len(in)) > nb {
		return in[0:nb] + "…"
	}
	return in
}

func dict(vals ...any) map[string]any {
	out := make(map[string]any)
	for i := 0; i < len(vals); i += 2 {
		k := vals[i].(string)
		v := vals[i+1]
		out[k] = v
	}
	return out
}

func pct(v1, v2 int64) int64 {
	return int64(float64(v1) / float64(v2) * 100)
}

func cents(cents int64) string {
	return fmt.Sprintf("$%0.2f", float64(cents)/100)
}

// Display last 4 digits of a CC
func last4(cc database.EncryptedString) string {
	return utils.Last4(string(cc))
}

func toMs(dur time.Duration) int64 {
	return int64(dur / time.Millisecond)
}

func derefI64(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}

func derefUserID(v *database.UserID) database.UserID {
	if v == nil {
		return 0
	}
	return *v
}

func shortHost(v string) string {
	parsed, err := url.Parse(v)
	if err != nil {
		return "unknown"
	}
	return strings.TrimPrefix(parsed.Host, "www.")
}

func b64(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}

func fmtBool(v bool) template.HTML {
	if v {
		return `<span style="color: green;">Y</span>`
	}
	return `<span style="color: red;">N</span>`
}

func isStrEmpty(v string) bool {
	return v == ""
}

func ms2s(v int64) int64 {
	return v / 1000
}

func since(v time.Time) string {
	return humanize.Time(v)
}
