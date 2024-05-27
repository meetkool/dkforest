package odometer

import (
	"dkforest/pkg/utils"
	"fmt"
)

type Odometer struct {
	s string
}

func New(s string) *Odometer {
	return &Odometer{s: s}
}

func (o *Odometer) Html() string {
	out := `<div class="odometer">`
	for i := 0; i < len(o.s); i++ {
		out += fmt.Sprintf(`<div class="wrapper"><div class="outer"><span class="below odometer%d"></span><span class="top inner">%c</span></div></div>`, i, o.s[i])
	}
	out += "</div>"
	return out
}

func (o *Odometer) Css() string {
	out := `
.odometer               { display: table; font-family: monospace; font-size: 40px; }
.odometer .wrapper      { display: table-cell; }
.odometer .outer        { display: grid; grid-template: 1fr / 1fr; place-items: center; }
.odometer .outer > *    { grid-column: 1 / 1; grid-row: 1 / 1; }
.odometer .outer .below { z-index: 1; }
.odometer .outer .top   { z-index: 2; }
.odometer .outer .inner { visibility: visible; animation-name: inner_frames; animation-duration: 3s; }
@keyframes inner_frames { 0% { visibility: hidden; } 99% { visibility: hidden; } 100% { visibility: visible; } }`
	l := len(o.s)
	for i := 0; i < l; i++ {
		out += fmt.Sprintf(`
.odometer .odometer%d:before { visibility: hidden; content: "%c"; animation-name: odometer%d_frames; animation-duration: 3s; }
@keyframes odometer%d_frames {`, i, o.s[i], i, i)
		out += `  0% { visibility: visible; }`
		n := 20
		step := 100.0 / float64(n)
		pct := 0.0
		var prev int
		for j := 0; j <= n; j++ {
			if j == n-(l-i) {
				break
			}
			if pct >= 95 {
				break
			}
			if j == 0 {
				pct += utils.RandFloat(0, 3)
			}
			var num int
			for {
				num = utils.RandInt(0, 9)
				if j > 0 && num == prev {
					continue
				}
				break
			}
			prev = num
			out += fmt.Sprintf(`%.2f%% { content: "%d"; }`, pct, num)
			pct += step
		}
		out += `  99% { visibility: visible; }`
		out += `  100% { visibility: hidden; }`
		out += `}`
	}
	return out
}
