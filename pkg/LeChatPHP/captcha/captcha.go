package captcha

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"math"
	"os"
	"sort"
	"strings"
)

const (
	b64Prefix = "R0lGODlhCAAOAIAAAAQCBPz-_CwAAAAACAAOAAAC"
	alphabet  = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	alphabet1 = "abdcefgh1ijkImnpoqrstyQuvwxzABCDEGJKMNHLORPFSTlUVWXYZ023456789"
)

var (
	onColor  = color.RGBA{R: 252, G: 254, B: 252, A: 255}
	redColor = color.RGBA{R: 204, G: 2, B: 4, A: 255}
	offColor = color.RGBA{R: 4, G: 2, B: 4, A: 255}
)

var b64Map = map[string]string{
	"D_AxdTP8lWthViTno4yvAgA7":             "z",
	"EPAxdTXcDYyvhbokkC8znwoAOw==":         "c",
	"EfAxFG3ZU2AAntrky1Hf-cECADs=":         "l",
	"EfAxdTP8gUMvLCoVnfVW9pcCADs=":         "s",
	"EfAxdTU8jjNSxnfvqqBi9pkCADs=":         "e",
	"EvAxdTECnDNxPplue3hr9kGjAAA7":         "r",
	"E_ARIlK523poxXlqyvRKCcEQKAAAOw==":     "i",
	"E_ARInL53EMrKQmvoS3bGbtKrAoAOw==":     "1",
	"E_ARYiO103IyQkQvtFTiZ51QPAoAOw==":     "J",
	"E_AtAlV5HYiyIWrvm9FN6j1MFAsAOw==":     "X",
	"E_AtAlW5nzuN1rgmzDLpjsBQNAoAOw==":     "H",
	"E_AtAnNZGYDPoWnruTo39aNQRAoAOw==":     "L",
	"E_AtdAMUGVySzYdcw1fi7MFQLAAAOw==":     "P",
	"E_AtdAOznYrwSGQrfQ7gzDROHAsAOw==":     "Z",
	"E_AtdBUzHGhonvrclZDyHsFQLAAAOw==":     "7",
	"E_AtdBkzHFCRTQmdRfrwF8FQLAAAOw==":     "F",
	"E_AtdPUawUPSBRgtpCnPXq9QvAoAOw==":     "T",
	"E_AvVI07UILIvRkrvbK6laNQRAoAOw==":     "3",
	"E_AvdFHR3IstPDBTlZrSxa1QBAoAOw==":     "I",
	"E_AxL0IZWWxrwkrdNQ_wHpVQRAoAOw==":     "t",
	"E_AxdTEC3IPOUHqirLFx7DdGXAoAOw==":     "n",
	"E_AxdTEgHHyRSgvpvDrj7phQKQAAOw==":     "m",
	"E_AxdTGi3TsO0CqtvXrL6ZlwKQAAOw==":     "v",
	"E_AxdTGi3TtRGgodBHnz2JlwKQAAOw==":     "u",
	"E_AxdTGi3TvxOUBvjTlTbDlGXAoAOw==":     "w",
	"E_AxdTGinzNyugeTvYBKWplwKQAAOw==":     "x",
	"E_AxdTU8jjNSxocetKBWfpmQKQAAOw==":     "o",
	"E_AxdTXcDYyh0XflzA_cTplQKQAAOw==":     "a",
	"E_DxIhK24WLp0Ykqvorl5ycpgwoAOw==":     "j",
	"FPAtAlV5HYiyIWqvnWenpT9MHJECADs=":     "Y",
	"FPAtAlW5nzttOQoszJRL_pBQHI0CADs=":     "W",
	"FPAtAlW5nzttTWhluhrITJFQHJECADs=":     "U",
	"FPAtdAMUU4NvMkkRPjpCZyVPHI0CADs=":     "E",
	"FPAvdEFxAzJONhgrfXhra6FKHJECADs=":     "S",
	"FPAxNE7NXWMIREntmzdtvRZKHMUCADs=":     "2",
	"FfARInL3nmLROAYdsjtjKr8vGUmjAAA7":     "A",
	"FfARInL3nmLROAYdsnpeurlEHEmgAAA7":     "0",
	"FfARIxK2n3uTyVXNxXbTHC-pGcmjAAA7":     "d",
	"FfARYuPWnmoSThQrszRht6tEHEmjAAA7":     "f",
	"FfAtAlW5H3By0VTnpHrXwy9EHEmxAAA7":     "V",
	"FfAtAnNZGYDK0SZdg4dmvDUPjMiyAAA7":     "h",
	"FfAtdDNOAXysxSWl1TPbDK8OGcmyAAA7":     "B",
	"FfAvAk0ZHItzRahWwshZ2rLrGcmjAAA7":     "k",
	"FfAxNE7NXVtpyvgqvgAy7r8NGcmyAAA7":     "O",
	"FfAxNE7NXVuJxQlmpHhb9i5EHEmjAAA7":     "9",
	"FfAxVI170YvApXBWpRDf6rJrGsmjAAA7":     "G",
	"FfAxVI17EYDRpRDlpfrc6pYKG0mgAAA7":     "C",
	"FfAxdTEC3IPOUHqirLHlPSfp6hilAAA7":     "p",
	"FfAxdTGi3TtRGgodBHnz2CvrobilAAA7":     "y",
	"FfAxdTPcwTsPGlhjsHQverVJ-8KlAAA7":     "g",
	"FfAxdTUc3oNOGhrQzVhSG7GkOkakAAA7":     "q",
	"FfDxInJZHQTL0MnsvS6iLJsnGcmkAAA7":     "4",
	"FvAtAlXPHYhwpcbuW1HWWbtQQ8jSNAoAOw==": "K",
	"FvAtAnNZGYDK0SZdg4dmbLEQjMjyKAAAOw==": "b",
	"FvAtArWvHYAqOQrlwnlzWqMKGcnSKAAAOw==": "N",
	"FvAtArWzYQRqOgprpQdz57cHGcnSKAAAOw==": "M",
	"FvAtdAMUGVySzXqeaxq3nKYOGcnSKAAAOw==": "R",
	"FvAtdAMUU4NvRfmgk4jxhjEtzMjSKAAAOw==": "5",
	"FvAtdDNOAXxsymYjva7ynBcPGckSKAAAOw==": "D",
	"FvAxNE7NXQMxsfUiZO3yuallQ8jSNAoAOw==": "6",
	"FvAxNE7NXVtpyvgmrIvFywEGGckSKAAAOw==": "Q",
	"FvAxNE7NXWMxgsUehjhXOzXuQsjSRAoAOw==": "8",
}

// SolveBase64 solve a base64 encoded gif
func SolveBase64(b64Str string) (string, error) {
	b64Str = strings.TrimPrefix(b64Str, "data:image/gif;base64,")
	b64, err := base64.StdEncoding.DecodeString(b64Str)
	if err != nil {
		return "", err
	}
	captchaImg, err := gif.Decode(bytes.NewReader(b64))
	if err != nil {
		return "", err
	}
	if captchaImg.Bounds().Max.X > 60 {
		return SolveDifficulty3(captchaImg)
	}
	return SolveDifficulty2(captchaImg)
}

// Solve a captcha difficulty 1
// Slice the captcha into 5, and directly compare the slices with our base64 hashmap
func Solve(img image.Image) (answer string, err error) {
	gifImg, ok := img.(*image.Paletted)
	if !ok {
		return "", errors.New("invalid gif image")
	}
	letterSize := image.Point{X: 8, Y: 14}
	cornerPt := image.Point{X: 5, Y: 7}
	for i := 0; i < 5; i++ {
		rect := image.Rectangle{Min: cornerPt, Max: cornerPt.Add(letterSize)}
		letterImg := gifImg.SubImage(rect)
		buf := bytes.Buffer{}
		_ = gif.Encode(&buf, letterImg, nil)
		letterB64 := base64.URLEncoding.EncodeToString(buf.Bytes())
		character, err := getCharByB64(letterB64)
		if err != nil {
			return "", fmt.Errorf("failed to get character %d", i+1)
		}
		answer += character
		cornerPt.X += letterSize.X + 1
	}
	return
}

// SolveDifficulty2 solve all captcha up to difficulty 2
// For difficulty 2, we slice the captcha into 5 slices, and compare
// which of our base64 images has the best match for the slice.
func SolveDifficulty2(img image.Image) (answer string, err error) {
	gifImg, ok := img.(*image.Paletted)
	if !ok {
		return "", errors.New("invalid gif image")
	}
	letterSize := image.Point{X: 8, Y: 14}
	cornerPt := image.Point{X: 5, Y: 7}
	for i := 0; i < 5; i++ {
		rect := image.Rectangle{Min: cornerPt, Max: cornerPt.Add(letterSize)}
		letterImg := gifImg.SubImage(rect).(*image.Paletted)
		character := ""
	alphabetLoop:
		for _, c := range alphabet1 {
			goodLetterImg, _ := getLetterImg(string(c))
			for y := 0; y < 14; y++ {
				for x := 0; x < 8; x++ {
					if goodLetterImg.At(x, y) == onColor && letterImg.At(cornerPt.X+x, cornerPt.Y+y) != onColor {
						continue alphabetLoop
					}
				}
			}
			character = string(c)
			break
		}
		// This should never happen
		if character == "" {
			return "", errors.New("failed to solve captcha")
		}
		answer += character
		cornerPt.X += letterSize.X + 1
	}
	return
}

// Count pixels that are On (either white or red)
func countPxOn(img *image.Paletted) (countOn int) {
	for y := 0; y < 14; y++ {
		for x := 0; x < 8; x++ {
			c := img.At(img.Bounds().Min.X+x, img.Bounds().Min.Y+y)
			if c == onColor || c == redColor {
				countOn += 1
			}
		}
	}
	return
}

// Count pixels that are red
func countRedPx(img *image.Paletted, offset image.Point) (countOn int) {
	for y := 0; y < img.Rect.Bounds().Dy(); y++ {
		for x := 0; x < img.Rect.Bounds().Dx(); x++ {
			c := img.At(offset.X+x, offset.Y+y)
			if c == redColor {
				countOn += 1
			}
		}
	}
	return
}

type Letter struct {
	Char      string
	Rect      image.Rectangle
	angles    []float64
	neighbors []Letter
}

func (l Letter) String() string {
	return fmt.Sprintf(`["%s",X:%d,Y:%d]`, l.Char, l.Center().X, l.Center().Y)
}

func (l Letter) Center() image.Point {
	return image.Point{X: l.Rect.Min.X + 4, Y: l.Rect.Min.Y + 6}
}

func (l Letter) Key() string {
	return fmt.Sprintf("%s_%d_%d", l.Char, l.Rect.Min.X, l.Rect.Min.Y)
}

func hasRedInCenterArea(letterImg *image.Paletted) bool {
	for y := 5; y <= 7; y++ {
		for x := 3; x <= 5; x++ {
			letterImgColor := letterImg.At(letterImg.Bounds().Min.X+x, letterImg.Bounds().Min.Y+y)
			if letterImgColor == redColor {
				return true
			}
		}
	}
	return false
}

// give an image and a valid letter image, return either or not the letter is in that image.
func imgContainsLetter(img, letterImg1 *image.Paletted) bool {
	for y := 0; y < letterImg1.Bounds().Size().Y; y++ {
		for x := 0; x < letterImg1.Bounds().Size().X; x++ {
			goodLetterColor := letterImg1.At(x, y)
			letterImgColor := img.At(img.Bounds().Min.X+x, img.Bounds().Min.Y+y)
			// If we find an Off pixel where it's supposed to be On, skip that letter
			if (goodLetterColor == onColor || goodLetterColor == redColor) &&
				(letterImgColor != onColor && letterImgColor != redColor) {
				return false
			}
		}
	}
	return true
}

func getContourRedPixels(img *image.Paletted) (out []image.Point) {
	topLeftPt := img.Bounds().Min
	bottomRightPt := img.Bounds().Max
	for i := 0; i < img.Bounds().Dx(); i++ {
		pxColor := img.At(topLeftPt.X+i, topLeftPt.Y)
		if pxColor == redColor {
			out = append(out, image.Point{X: topLeftPt.X + i, Y: topLeftPt.Y})
		}
		pxColor = img.At(topLeftPt.X+i, bottomRightPt.Y-1)
		if pxColor == redColor {
			out = append(out, image.Point{X: topLeftPt.X + i, Y: bottomRightPt.Y - 1})
		}
	}
	for i := 1; i < img.Bounds().Dy()-1; i++ {
		pxColor := img.At(topLeftPt.X, topLeftPt.Y+i)
		if pxColor == redColor {
			out = append(out, image.Point{X: topLeftPt.X, Y: topLeftPt.Y + i})
		}
		if img.Bounds().Dx() < img.Bounds().Dy() {
			if img.Bounds().Max.X == 150 {
				continue
			}
		}
		pxColor = img.At(bottomRightPt.X-1, topLeftPt.Y+i)
		if pxColor == redColor {
			out = append(out, image.Point{X: bottomRightPt.X - 1, Y: topLeftPt.Y + i})
		}
	}
	return
}

func getAngle(p1, p2 image.Point) float64 {
	return math.Atan2(float64(p1.Y-p2.Y), float64(p1.X-p2.X))
}

func getLetterInDirection(letter *Letter, angle float64, lettersMap LettersCache) (out *Letter) {
	minAngle := math.MaxFloat64
	// Visit every other letters
	for _, otherLetter := range lettersMap.toSlice() {
		if otherLetter.Key() == letter.Key() {
			continue
		}
		// Find the angle between the two letters
		t := getAngle(otherLetter.Center(), letter.Center())
		if t < 0 {
			t += 2 * math.Pi
		}
		if angle < 0 {
			angle += 2 * math.Pi
		}
		angleDiff := math.Abs(angle - t)
		if angleDiff < minAngle {
			// Keep track of the letter with the smaller angle difference
			minAngle = angleDiff
			out = otherLetter
		}
	}
	return
}

type LettersCache map[string]*Letter

func (c LettersCache) toSlice() (out []*Letter) {
	for _, v := range c {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return
}

// To find orientation of ordered triplet (p, q, r).
// The function returns following values
// 0 --> p, q and r are collinear
// 1 --> Clockwise
// 2 --> Counterclockwise
func orientation(p, q, r image.Point) int {
	// See http://www.geeksforgeeks.org/orientation-3-ordered-points/
	// for details of below formula.
	collinear := 0
	clockwise := 1
	counterclockwise := 2

	val := (q.Y-p.Y)*(r.X-q.X) - (q.X-p.X)*(r.Y-q.Y)
	fmt.Println("TEST", val)
	if val == 0 { // collinear
		return collinear
	}
	// clock or counterclockwise
	if val > 0 {
		return clockwise
	}
	return counterclockwise
}

// SolveDifficulty3 solve captcha for difficulty 3
// For each pixel, verify if a match is found. If we do have a match,
// verify that we have some "red" in it.
// TODO: figure out how to get the right order
//
// Red circle is 17x17 (initial point)
func SolveDifficulty3(img image.Image) (answer string, err error) {
	gifImg, ok := img.(*image.Paletted)
	if !ok {
		return "", errors.New("invalid gif image")
	}

	imageWidth := 150
	imageHeight := 200
	letterSize := image.Point{X: 8, Y: 14}
	minPxForLetter := 21
	minStartingPtRedPx := 50

	var starting *Letter             // Hold the starting letter
	lettersMap := make(LettersCache) // Build a hashmap to quickly access letters

	// Step1: Find all letters with red on the center
	for y := 0; y < imageHeight; y++ {
		for x := 0; x < imageWidth; x++ {
			topLeftPt := image.Point{X: x, Y: y}
			rect := image.Rectangle{Min: topLeftPt, Max: topLeftPt.Add(letterSize)}
			letterImg := gifImg.SubImage(rect).(*image.Paletted)

			// We know that minimum amount of pixels on to form a letter is 21
			// We can skip squares that do not have this prerequisite
			if countPxOn(letterImg) < minPxForLetter {
				continue
			}
			// Check middle pixels for red, if no red pixels, we can ignore that square
			if !hasRedInCenterArea(letterImg) {
				continue
			}

			for _, c := range alphabet1 {
				goodLetterImg, _ := getLetterImg(string(c))
				if !imgContainsLetter(letterImg, goodLetterImg) {
					continue
				}

				// "w" fits in "W". So if we find "W" 1 px bellow, discard "w"
				if c == 'w' {
					capitalWImg, _ := getLetterImg("W")
					newPt := topLeftPt.Add(image.Point{X: 0, Y: 1})
					rect := image.Rectangle{Min: newPt, Max: newPt.Add(letterSize)}
					onePxDownImg := gifImg.SubImage(rect).(*image.Paletted)
					if imgContainsLetter(onePxDownImg, capitalWImg) {
						continue
					}
				} else if c == 'k' {
					capitalKImg, _ := getLetterImg("K")
					newPt := topLeftPt.Add(image.Point{X: 1, Y: 1})
					rect := image.Rectangle{Min: newPt, Max: newPt.Add(letterSize)}
					onePxUpImg := gifImg.SubImage(rect).(*image.Paletted)
					if imgContainsLetter(onePxUpImg, capitalKImg) {
						continue
					}
				}

				letter := &Letter{Char: string(c), Rect: rect}
				lettersMap[letter.Key()] = letter // Keep letters in hashmap for easy access
				break
			}
		}
	}

	if len(lettersMap) != 5 {
		return "", fmt.Errorf("did not find exactly 5 letters (%d)", len(lettersMap))
	}

	// Step2: Find the starting letter
	for _, letter := range lettersMap.toSlice() {
		p1 := letter.Rect.Min.Add(image.Point{X: -5, Y: -3})
		p2 := letter.Rect.Max.Add(image.Point{X: 6, Y: 2})
		rect := image.Rectangle{Min: p1, Max: p2}
		square := gifImg.SubImage(rect).(*image.Paletted)

		// Find starting point
		redCount := countRedPx(square, p1)
		if redCount > minStartingPtRedPx {
			starting = letter
			break
		}
	}

	if starting == nil {
		return "", errors.New("could not find starting letter")
	}

	code := ""
	letter := starting
	visited := make(map[string]bool)
	for i := 0; i < 5; i++ {
		if _, found := visited[letter.Key()]; found {
			return "", errors.New("already visited node")
		}
		code += letter.Char
		visited[letter.Key()] = true
		if i == 4 {
			break
		}

		p1 := letter.Rect.Min.Add(image.Point{X: -5, Y: -3})
		p2 := letter.Rect.Max.Add(image.Point{X: 6, Y: 2})
		rect := image.Rectangle{Min: p1, Max: p2}

		retry := 0
	Loop:
		for {
			retry++
			square := gifImg.SubImage(rect).(*image.Paletted)
			//SaveGif("char_"+letter.Key()+".gif", square)
			redPxPts := getContourRedPixels(square)

			if i == 0 {
				if len(redPxPts) == 0 {
					if retry < 10 {
						rect.Min = rect.Min.Add(image.Point{X: -1, Y: -1})
						rect.Max = rect.Max.Add(image.Point{X: 1, Y: 1})
						continue
					}
					return "", fmt.Errorf("root %s has no line detected", letter)
				}
				if len(redPxPts) > 1 {
					return "", fmt.Errorf("root %s has more than one line detected", letter)
				}
				redPt := redPxPts[0]
				angle := getAngle(redPt, letter.Center())
				neighbor := getLetterInDirection(letter, angle, lettersMap)
				letter = neighbor
				break
			}

			if len(redPxPts) == 0 {
				if retry < 10 {
					rect.Min = rect.Min.Add(image.Point{X: -1, Y: -1})
					rect.Max = rect.Max.Add(image.Point{X: 1, Y: 1})
					continue
				}
				return "", fmt.Errorf("letter #%d %s has no line detected", i+1, letter)
			}
			if len(redPxPts) == 1 {
				if retry < 10 {
					rect.Min = rect.Min.Add(image.Point{X: -1, Y: -1})
					rect.Max = rect.Max.Add(image.Point{X: 1, Y: 1})
					continue
				}
				return "", fmt.Errorf("letter #%d %s has only 1 line detected", i+1, letter)
			}
			if len(redPxPts) > 2 {
				return "", fmt.Errorf("letter #%d %s has more than 2 lines detected", i+1, letter)
			}
			fstRedPt := redPxPts[0]
			angle := getAngle(fstRedPt, letter.Center())
			neighbor := getLetterInDirection(letter, angle, lettersMap)
			if _, found := visited[neighbor.Key()]; found {
				fstRedPt := redPxPts[1]
				angle := getAngle(fstRedPt, letter.Center())
				neighbor = getLetterInDirection(letter, angle, lettersMap)
				if _, found := visited[neighbor.Key()]; found {
					if i == 3 {
						for _, l := range lettersMap.toSlice() {
							if _, found := visited[l.Key()]; !found {
								letter = l
								break Loop
							}
						}
					}
					return "", fmt.Errorf("letter #%d %s all neighbors already visited", i+1, letter)
				}
			}
			letter = neighbor
			break
		}
	}

	answer = code
	return
}

// Given a base64 string, return the letter that match the gif
func getCharByB64(b64 string) (string, error) {
	b64 = strings.TrimPrefix(b64, b64Prefix)
	if v, found := b64Map[b64]; found {
		return v, nil
	}
	return "", errors.New("character not found")
}

// Given a letter (eg: "a") return the gif image for that letter
func getLetterImg(letter string) (*image.Paletted, error) {
	for k, v := range b64Map {
		if v == letter {
			goodLetter, _ := base64.URLEncoding.DecodeString(b64Prefix + k)
			img, _ := gif.Decode(bytes.NewReader(goodLetter))
			if palettedImg, ok := img.(*image.Paletted); ok {
				return palettedImg, nil
			}
			break
		}
	}
	return nil, errors.New("letter not found")
}

// SaveGif save an image on disk
func SaveGif(filename string, img image.Image) {
	f, _ := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY, 0644)
	_ = gif.Encode(f, img, nil)
	_ = f.Close()
}

// GetAlphabetImg generate an image of all the characters
func GetAlphabetImg() {
	rect := image.Rectangle{Min: image.Point{X: 0, Y: 0}, Max: image.Point{X: 5 + (62 * 9) + 5, Y: 8 + 14 + 8}}
	newImg := image.NewPaletted(rect, color.Palette{onColor, offColor})
	draw.Draw(newImg, newImg.Bounds(), &image.Uniform{C: color.Black}, image.Point{}, draw.Src)
	letterSize := image.Point{X: 8, Y: 14}
	cornerPt := image.Point{X: 5, Y: 8}
	for _, c := range alphabet {
		b64 := ""
		for k, v := range b64Map {
			if v == string(c) {
				b64 = k
				break
			}
		}
		letterB64, _ := base64.URLEncoding.DecodeString(b64Prefix + b64)
		letterImg, _ := gif.Decode(bytes.NewReader(letterB64))
		r := image.Rectangle{Min: cornerPt, Max: cornerPt.Add(letterSize)}
		draw.Draw(newImg, r, letterImg, image.Point{X: 0, Y: 0}, draw.Src)
		cornerPt.X += letterSize.X + 1
	}
	SaveGif("alphabet.gif", newImg)
}
