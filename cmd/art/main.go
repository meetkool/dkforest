package main

import (
	"fmt"
	"image/color"
	"image/png"
	"os"
	"sort"
	"strings"
)

func main() {

	in := `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`
	maskFile := "mask.png"
	outFile := "out.html"

	f, _ := os.Open(maskFile)
	im, _ := png.Decode(f)

	lines := strings.Split(in, "\n")
	w := len(lines[0])
	h := len(lines)

	classPrefix := "gpg_"
	prevClass := ""
	colorsMap := make(map[string]string)
	colorsArr := [][]string{}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := im.At(x, y)
			hex := HexFromRgbColor(c.(color.NRGBA))
			if _, ok := colorsMap[hex]; ok {
				continue
			}
			prevClass = getNextClassNameStr(prevClass)
			className := classPrefix + prevClass
			colorsMap[hex] = className
			colorsArr = append(colorsArr, []string{className, hex})
		}
	}
	sort.Slice(colorsArr, func(i, j int) bool {
		return colorsArr[i][0] < colorsArr[j][0]
	})

	out := ""
	out += "<style>\n"
	for _, v := range colorsArr {
		out += fmt.Sprintf(".%s{color:%s}\n", v[0], v[1])
	}
	out += "</style>\n"
	out += "<div style=\"font-family: monospace; background-color: #222;\">\n"
	out += "<pre>\n"
	spanOpen := false
	prevClass = ""
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := im.At(x, y)
			if len(lines[y]) <= x {
				break
			}
			char := lines[y][x]
			hex := HexFromRgbColor(c.(color.NRGBA))
			class := colorsMap[hex]
			if class != prevClass {
				prevClass = class
				if spanOpen {
					out += "</span>"
				}
				out += fmt.Sprintf(`<span class="%s">`, class)
				spanOpen = true
			}
			out += string(char)
		}
		out += "\n"
	}
	if spanOpen {
		out += "</span>"
	}
	out += "</pre>\n"
	out += "</div>"

	_ = os.WriteFile(outFile, []byte(out), 0644)
}

func HexFromRgbColor(color color.NRGBA) string {
	return fmt.Sprintf("#%X%X%X", color.R, color.G, color.B)
}

const firstRune = 'a'
const lastRune = 'z'

func getNextClassNameStr(in string) string {
	return string(getNextClassName([]rune(in)))
}

func getNextClassName(in []rune) (out []rune) {
	out = in
	for i := len(out) - 1; i >= 0; i-- {
		if out[i] < lastRune {
			out[i]++
			return
		}
		out[i] = firstRune
	}
	return append([]rune{firstRune}, out...)
}

func getClassName(idx int) string {
	out := []rune{firstRune}
	for counter := 0; counter < idx; counter++ {
		out = getNextClassName(out)
	}
	return string(out)
}
