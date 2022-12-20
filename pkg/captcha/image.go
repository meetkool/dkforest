// Copyright 2011-2014 Dmitry Chestnykh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package captcha

import (
	"bytes"
	"dkforest/bindata"
	"github.com/fogleman/gg"
	"github.com/golang/freetype/truetype"
	"github.com/sirupsen/logrus"
	font1 "golang.org/x/image/font"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"math/rand"
	"strconv"
	"sync"
)

type Image struct {
	*image.RGBA
	rnd              *rand.Rand
	imageWidth       int
	imageHeight      int
	numWidth         int
	numHeight        int
	nbDigits         int
	digitSpacing     int
	digitVertSpacing int
	borderTop        int
	borderLeft       int
	nbNumRows        int
	nbNumCols        int
	centerOffset     float64
	numbersMatrix    [][]Number
	mainPath         Path
	secondPath       Path
	thirdPath        Path
	displayDebug     bool
	renderHelpImg    bool
	difficulty       int64
	c                *gg.Context
	CubicDelta       float64
}

type Number struct {
	Num     int
	Angle   float64
	FaceIdx int
	Face    font1.Face
	Opacity uint8
}

type iPoint interface {
	GetX() float64
	GetY() float64
}

// To find orientation of ordered triplet (p, q, r).
// The function returns following values
// 0 --> p, q and r are collinear
// 1 --> Clockwise
// 2 --> Counterclockwise
func orientation(p, q, r iPoint) int {
	// See http://www.geeksforgeeks.org/orientation-3-ordered-points/
	// for details of below formula.
	collinear := 0
	clockwise := 1
	counterclockwise := 2

	val := (q.GetY()-p.GetY())*(r.GetX()-q.GetX()) - (q.GetX()-p.GetX())*(r.GetY()-q.GetY())
	if val == 0 { // collinear
		return collinear
	}
	// clock or counterclockwise
	if val > 0 {
		return clockwise
	}
	return counterclockwise
}

// The main function that returns true if line segment 'p1q1'
// and 'p2q2' intersect.
func doIntersect(p1, q1, p2, q2 iPoint) bool {
	// Find the four orientations needed for general and
	// special cases
	o1 := orientation(p1, q1, p2)
	o2 := orientation(p1, q1, q2)
	o3 := orientation(p2, q2, p1)
	o4 := orientation(p2, q2, q1)
	// General case
	if o1 != o2 && o3 != o4 {
		return true
	}
	// Special Cases
	// p1, q1 and p2 are collinear and p2 lies on segment p1q1
	if o1 == 0 && onSegment(p1, p2, q1) {
		return true
	}
	// p1, q1 and p2 are collinear and q2 lies on segment p1q1
	if o2 == 0 && onSegment(p1, q2, q1) {
		return true
	}
	// p2, q2 and p1 are collinear and p1 lies on segment p2q2
	if o3 == 0 && onSegment(p2, p1, q2) {
		return true
	}
	// p2, q2 and q1 are collinear and q1 lies on segment p2q2
	if o4 == 0 && onSegment(p2, q1, q2) {
		return true
	}
	return false // Doesn't fall in any of the above cases
}

// Find intersection point
func intersect(p0, p1, p2, p3 iPoint) Point {
	var intersect Point
	var s1 Point
	var s2 Point

	s1.pxY = p1.GetY() - p0.GetY()
	s1.pxX = p1.GetX() - p0.GetX()
	s2.pxY = p3.GetY() - p2.GetY()
	s2.pxX = p3.GetX() - p2.GetX()

	//s := (-s1.GetX()*(p0.GetY()-p2.GetY()) + s1.GetY()*(p0.GetX()-p2.GetX())) / (-s2.GetY()*s1.GetX() + s1.GetY()*s2.GetX())
	t := (s2.GetY()*(p0.GetX()-p2.GetX()) - s2.GetX()*(p0.GetY()-p2.GetY())) / (-s2.GetY()*s1.GetX() + s1.GetY()*s2.GetX())

	intersect.pxY = p0.GetY() + (t * s1.GetY())
	intersect.pxX = p0.GetX() + (t * s1.GetX())

	return intersect
}

// Given three collinear points p, q, r, the function checks if
// point q lies on line segment 'pr'
func onSegment(p, q, r iPoint) bool {
	return q.GetY() <= math.Max(p.GetY(), r.GetY()) && q.GetY() >= math.Min(p.GetY(), r.GetY()) &&
		q.GetX() <= math.Max(p.GetX(), r.GetX()) && q.GetX() >= math.Min(p.GetX(), r.GetX())
}

func colorDistance(e1, e2 color.RGBA) float64 {
	r := float64(e1.R - e2.R)
	g := float64(e1.G - e2.G)
	b := float64(e1.B - e2.B)
	return math.Sqrt(r*r + g*g + b*b)
}

func distance(p1, p2 iPoint) float64 {
	return math.Sqrt(math.Pow(p2.GetX()-p1.GetX(), 2) + math.Pow(p2.GetY()-p1.GetY(), 2))
}

func angle(p1, p2 iPoint) float64 {
	return math.Atan2(p1.GetY()-p2.GetY(), p1.GetX()-p2.GetX())
}

type Point struct {
	X   int
	Y   int
	pxX float64
	pxY float64
}

func (p Point) GetX() float64 {
	return p.pxX
}

func (p Point) GetY() float64 {
	return p.pxY
}

// Get an int in range [from, to] which is different from "value"
func (m *Image) getDifferentInt(value, from, to int) (out int) {
	for {
		out = m.RandInt(from, to)
		if out != value {
			return
		}
	}
}

type Path struct {
	points []Point
}

func (p Path) coordTaken(row, col int) bool {
	for _, pt := range p.points {
		if pt.Y == row && pt.X == col {
			return true
		}
	}
	return false
}

func (p Path) calculateNbIntersections() (nbIntersects int) {
	// Calculate nb intersections
	for i := 3; i < len(p.points); i++ {
		curr := p.points[i]
		prev := p.points[i-1]
		p1 := p.points[0]
		for ii := 1; ii < i-1; ii++ {
			p2 := p.points[ii]
			if doIntersect(curr, prev, p1, p2) {
				nbIntersects++
			}
			p1 = p2
		}
	}
	return
}

func (p Path) checkIntersections() bool {
	// Calculate nb intersections
	for i := 3; i < len(p.points); i++ {
		curr := p.points[i]
		p1 := p.points[0]
		for ii := 1; ii < i-1; ii++ {
			p2 := p.points[ii]
			if doIntersect(curr, curr, p1, p2) {
				return false
			}
			p1 = p2
		}
	}
	return true
}

func (p Path) checkOverlapping() bool {
	// Calculate nb intersections
	for i := 2; i < len(p.points); i++ {
		curr := p.points[i]
		p1 := p.points[i-1]
		p2 := p.points[i-2]
		if findAngle(curr, p1, p2) == 0 {
			return false
		}
		if orientation(curr, p1, p2) == 0 {
			return false
		}
		if onSegment(p1, p2, curr) {
			return false
		}
	}
	return true
}

func findAngle(p0, p1, c iPoint) float64 {
	p0c := math.Sqrt(math.Pow(c.GetX()-p0.GetX(), 2) + math.Pow(c.GetY()-p0.GetY(), 2))
	p1c := math.Sqrt(math.Pow(c.GetX()-p1.GetX(), 2) + math.Pow(c.GetY()-p1.GetY(), 2))
	p0p1 := math.Sqrt(math.Pow(p1.GetX()-p0.GetX(), 2) + math.Pow(p1.GetY()-p0.GetY(), 2))
	x := (p1c*p1c + p0c*p0c - p0p1*p0p1) / (2 * p1c * p0c)
	if x > 1 {
		x = 1
	}
	if x < -1 {
		x = -1
	}
	return math.Acos(x)
}

func (p Path) intersectWith(p1, p2 iPoint) bool {
	// Calculate nb intersections
	for i := 1; i < len(p.points); i++ {
		curr := p.points[i]
		prev := p.points[i-1]
		if doIntersect(curr, prev, p1, p2) {
			return true
		}
	}
	return false
}

func (m *Image) generateSemiValidPath(usedCoordsMap map[Point]struct{}) Path {
	path := Path{}
Loop:
	for {
		path = Path{}
		path.points = make([]Point, 0)
		row := -1
		col := -1
		var prevCoord Point
		var prevTmp Point
		for i := 0; i < m.nbDigits; i++ {
			tmp := 0
			for {
				tmp++
				if tmp > 100 {
					continue Loop
				}
				prevTmp = prevCoord
				row = m.getDifferentInt(prevTmp.Y, 0, 3)
				col = m.getDifferentInt(prevTmp.X, 0, 4)
				if path.coordTaken(row, col) {
					continue
				}
				if _, found := usedCoordsMap[m.createPoint(col, row)]; found {
					continue
				}
				curr := m.createPoint(col, row)
				path.points = append(path.points, curr)
				prevCoord = curr
				break
			}
		}
		break
	}
	return path
}
func (m *Image) generateValidPath(usedCoordsMap map[Point]struct{}) Path {
	path := Path{}
	for {
		path = m.generateSemiValidPath(usedCoordsMap)
		if !path.checkIntersections() {
			continue
		}
		if !path.checkOverlapping() {
			continue
		}
		nbIntersects := path.calculateNbIntersections()
		// Acceptable captcha if we have at least 1 intersection
		if nbIntersects > 1 {
			break
		}
	}
	return path
}

func (m *Image) createPoint(col, row int) Point {
	return Point{X: col, Y: row,
		pxX: float64(m.borderLeft+col*(m.numWidth+m.digitSpacing)) + (float64(m.numWidth) / 2) + m.RandFloat(-m.centerOffset, m.centerOffset),
		pxY: float64(m.borderTop+row*(m.numHeight+m.digitVertSpacing)) + (float64(m.numHeight) / 2) + m.RandFloat(-m.centerOffset, m.centerOffset)}
}

func (m *Image) RandInt(min, max int) int {
	if min == max {
		return min
	}
	if max < min {
		min, max = max, min
	}
	return int(m.rnd.Int63n(int64(max-min+1))) + min
}

func (m *Image) RandFloat(min, max float64) float64 {
	if min == max {
		return min
	}
	if max < min {
		min, max = max, min
	}
	return m.rnd.Float64()*(max-min) + min
}

var signatureFace = loadSignatureFace()
var faces = loadFontFaces()

func NewImage(digits []byte, difficulty int64, rnd *rand.Rand) *Image {
	//start := time.Now()
	//defer func() { fmt.Println("took:", time.Since(start)) }()

	m := new(Image)
	m.rnd = rnd
	m.renderHelpImg = false
	m.displayDebug = false
	m.imageWidth = 240
	m.imageHeight = 120
	m.numWidth = 11
	m.numHeight = 18
	m.nbNumRows = 4
	m.nbNumCols = 5
	m.centerOffset = 0.0
	m.nbDigits = 6
	m.borderTop = 7
	m.borderLeft = 25
	m.digitSpacing = 35
	m.digitVertSpacing = 12
	m.difficulty = difficulty

	if m.difficulty == 1 {
		m.CubicDelta = 0
	} else {
		m.CubicDelta = 20
	}

	updateUsedCoordsMap := func(path Path, usedCoordsMap map[Point]struct{}) {
		for _, pt := range path.points {
			usedCoordsMap[pt] = struct{}{}
		}
	}

	usedCoordsMap := make(map[Point]struct{})
	m.mainPath = m.generateValidPath(usedCoordsMap)
	updateUsedCoordsMap(m.mainPath, usedCoordsMap)
	m.secondPath = m.generateSemiValidPath(usedCoordsMap)
	updateUsedCoordsMap(m.secondPath, usedCoordsMap)
	m.thirdPath = m.generateSemiValidPath(usedCoordsMap)
	updateUsedCoordsMap(m.thirdPath, usedCoordsMap)

	// Generate all numbers
	m.numbersMatrix = make([][]Number, 0)
	for row := 0; row < m.nbNumRows; row++ {
		numbers := make([]Number, 0)
		for col := 0; col < m.nbNumCols; col++ {
			d := m.RandInt(0, 9)
			opacity := uint8(255)
			if m.renderHelpImg {
				opacity = 100
			}
			faceIdx := m.RandInt(0, len(faces)-1)
			numbers = append(numbers, Number{
				Num:     d,
				Angle:   m.RandFloat(-40, 40),
				FaceIdx: faceIdx,
				Face:    faces[faceIdx],
				Opacity: opacity})
		}
		m.numbersMatrix = append(m.numbersMatrix, numbers)
	}

	// Replace numbers by the answer digits
	for i, c := range m.mainPath.points {
		d := int(digits[i])
		// 7 with negative angle looks like 1
		if d == 7 {
			m.numbersMatrix[c.Y][c.X].Angle = m.RandFloat(0, 40)
		}
		m.numbersMatrix[c.Y][c.X].Opacity = 255
		m.numbersMatrix[c.Y][c.X].Num = d
	}

	m.render()
	return m
}

func (m *Image) getLineColor() color.RGBA {
	min := 80
	max := 255
	return color.RGBA{R: uint8(m.RandInt(min, max)), G: uint8(m.RandInt(min, max)), B: uint8(m.RandInt(min, max)), A: 255}
}

func loadFontFaces() (out []font1.Face) {
	loadFF := func(path string, size float64) {
		fontBytes := bindata.MustAsset(path)
		f, err := truetype.Parse(fontBytes)
		if err != nil {
			logrus.Error(err)
		}
		face := truetype.NewFace(f, &truetype.Options{Size: size})
		out = append(out, face)
	}
	loadFF("font/Lato-Regular.ttf", 25)
	loadFF("font/JessicaGroovyBabyFINAL2.ttf", 25)
	loadFF("font/PineappleDelight.ttf", 25)
	loadFF("font/df66c.ttf", 20)
	loadFF("font/agengsans.ttf", 30)
	return
}

func loadSignatureFace() font1.Face {
	fontBytes := bindata.MustAsset("font/Lato-Regular.ttf")
	f, _ := truetype.Parse(fontBytes)
	return truetype.NewFace(f, &truetype.Options{Size: 10})
}

func (m *Image) withState(clb func()) {
	m.c.Push()
	defer m.c.Pop()
	clb()
}

// This is a hack because github.com/golang/freetype is not thread safe and will crash the program.
// So we can only render 1 captcha at the time.
// https://github.com/golang/freetype/issues/65
// Can reproduce the bug by launching two terminals that runs the following script
/**
#!/bin/bash
for i in {1..100}
do
   curl -s "http://127.0.0.1:8080/bhc?username=n0tr1v" > /dev/null
done
*/
var renderMutex = sync.Mutex{}

func (m *Image) render() {
	m.RGBA = image.NewRGBA(image.Rect(0, 0, m.imageWidth, m.imageHeight))
	m.c = gg.NewContextForRGBA(m.RGBA)

	// This hack should never be necessary since we have the renderMutex
	defer func() {
		if r := recover(); r != nil {
			logrus.Error("Failed to create captcha (font concurrency bug)")
		}
	}()

	renderMutex.Lock()
	defer renderMutex.Unlock()
	m.withState(func() {
		m.renderBackground()
		m.renderTrianglesPattern()
		//m.renderUselessLines()
		m.renderDigits(0, color.RGBA{R: 255, G: 255, B: 255, A: 255})
		m.renderOrphanNumbers()
		m.renderFakePath(m.secondPath.points)
		m.renderFakePath(m.thirdPath.points)
		m.renderPath(m.mainPath.points)
		m.renderDigits(1, color.RGBA{R: 255, G: 255, B: 255, A: 100})
		m.renderDebugGrid()
		m.renderSignature()
		m.renderBorder()
	})
}

func (m *Image) renderOrphanNumbers() {
	points := make([]Point, 0)
	for row := 0; row < m.nbNumRows; row++ {
		for col := 0; col < m.nbNumCols; col++ {
			if m.mainPath.coordTaken(row, col) {
				continue
			}
			if m.secondPath.coordTaken(row, col) {
				continue
			}
			if m.thirdPath.coordTaken(row, col) {
				continue
			}
			pt := m.createPoint(col, row)
			points = append(points, pt)
		}
	}
	m.renderFakePath(points)
}

func (m *Image) renderUselessLines() {
	d := 20.0
	m.withState(func() {
		for i := 0; i < 3; i++ {
			if m.RandInt(0, 1) == 0 {
				d *= -1
			}
			pt1 := Point{pxX: 0, pxY: m.RandFloat(-50, 30+float64(m.imageHeight))}
			pt2 := Point{pxX: float64(m.imageWidth), pxY: m.RandFloat(-30, 50+float64(m.imageHeight))}
			m.renderPath([]Point{pt1, pt2})
		}
		for i := 0; i < 3; i++ {
			if m.RandInt(0, 1) == 0 {
				d *= -1
			}
			pt1 := Point{pxX: 0, pxY: m.RandFloat(-30, 30+float64(m.imageHeight))}
			pt2 := Point{pxX: float64(m.imageWidth), pxY: m.RandFloat(-30, 30+float64(m.imageHeight))}
			grad := gg.NewLinearGradient(pt1.GetX(), pt1.GetY(), pt2.GetX(), pt2.GetY())
			grad.AddColorStop(0, m.getLineColor())
			grad.AddColorStop(0.5, m.getLineColor())
			grad.AddColorStop(1, m.getLineColor())
			m.c.SetStrokeStyle(grad)
			m.c.SetDash(5, 3)
			m.c.SetLineWidth(1)
			m.c.MoveTo(pt1.GetX(), pt1.GetY())
			m.c.CubicTo(pt1.GetX()+d, pt1.GetY()+d, pt2.GetX()-d, pt2.GetY()-d, pt2.GetX(), pt2.GetY())
			m.c.Stroke()
		}
		for i := 0; i < 3; i++ {
			if m.RandInt(0, 1) == 0 {
				d *= -1
			}
			pt1 := Point{pxY: 0, pxX: m.RandFloat(-30, 30+float64(m.imageWidth))}
			pt2 := Point{pxY: float64(m.imageHeight), pxX: m.RandFloat(-30, 30+float64(m.imageHeight))}
			grad := gg.NewLinearGradient(pt1.GetX(), pt1.GetY(), pt2.GetX(), pt2.GetY())
			grad.AddColorStop(0, m.getLineColor())
			grad.AddColorStop(0.5, m.getLineColor())
			grad.AddColorStop(1, m.getLineColor())
			m.c.SetStrokeStyle(grad)
			m.c.SetDash(5, 3)
			m.c.SetLineWidth(1)
			m.c.MoveTo(pt1.GetX(), pt1.GetY())
			m.c.CubicTo(pt1.GetX()+d, pt1.GetY()+d, pt2.GetX()-d, pt2.GetY()-d, pt2.GetX(), pt2.GetY())
			m.c.Stroke()
		}
	})
}

func (m *Image) renderPath(points []Point) {
	d := m.CubicDelta

	gradColors := make([]color.RGBA, 0)
	for i := 0; i < len(points); i++ {
		gradColors = append(gradColors, m.getLineColor())
		gradColors = append(gradColors, m.getLineColor())
	}

	m.withState(func() {
		// Draw the whole line
		m.withState(func() {
			startColor := gradColors[0]
			for i := 1; i < len(points); i++ {
				prev := points[i-1]
				pt := points[i]

				midColor := gradColors[(i*2)-1]
				lastColor := gradColors[i*2]
				grad := gg.NewLinearGradient(prev.GetX(), prev.GetY(), pt.GetX(), pt.GetY())
				grad.AddColorStop(0, startColor)
				grad.AddColorStop(0.5, midColor)
				grad.AddColorStop(1, lastColor)
				startColor = lastColor
				m.c.SetStrokeStyle(grad)
				//m.c.SetColor(color.White)

				m.withState(func() {
					m.c.SetLineWidth(4)
					m.c.SetColor(color.RGBA{R: 15, G: 15, B: 15, A: 255})
					m.c.MoveTo(prev.GetX(), prev.GetY())
					m.c.CubicTo(prev.GetX()+d, prev.GetY()+d, pt.GetX()-d, pt.GetY()-d, pt.GetX(), pt.GetY())
					m.c.Stroke()
				})

				m.withState(func() {
					m.c.SetLineWidth(1.5)
					m.c.MoveTo(prev.GetX(), prev.GetY())
					m.c.CubicTo(prev.GetX()+d, prev.GetY()+d, pt.GetX()-d, pt.GetY()-d, pt.GetX(), pt.GetY())
					m.c.Stroke()
				})
			}
		})

		m.withState(func() {
			m.c.SetLineWidth(1.5)
			m.c.SetDash(10, 300)
			startColor := gradColors[0]
			for i := 1; i < len(points); i++ {
				prev := points[i-1]
				pt := points[i]

				midColor := gradColors[(i*2)-1]
				lastColor := gradColors[i*2]
				grad := gg.NewLinearGradient(prev.GetX(), prev.GetY(), pt.GetX(), pt.GetY())
				grad.AddColorStop(0, startColor)
				grad.AddColorStop(0.5, midColor)
				grad.AddColorStop(1, lastColor)
				startColor = lastColor
				m.c.SetStrokeStyle(grad)
				//m.c.SetColor(color.RGBA{255, 0, 0, 255})

				m.c.MoveTo(prev.GetX(), prev.GetY())
				m.c.CubicTo(prev.GetX()+d, prev.GetY()+d, pt.GetX()-d, pt.GetY()-d, pt.GetX(), pt.GetY())
				m.c.Stroke()

				if i == len(points)-1 {
					m.withState(func() {
						m.c.SetLineWidth(4)
						m.c.SetColor(color.RGBA{R: 15, G: 15, B: 15, A: 255})
						m.c.MoveTo(pt.GetX(), pt.GetY())
						m.c.CubicTo(pt.GetX()-d, pt.GetY()-d, prev.GetX()+d, prev.GetY()+d, prev.GetX(), prev.GetY())
						m.c.Stroke()
					})

					m.withState(func() {
						m.c.SetDashOffset(4)
						m.c.SetDash(30, 300)
						m.c.MoveTo(pt.GetX(), pt.GetY())
						m.c.CubicTo(pt.GetX()-d, pt.GetY()-d, prev.GetX()+d, prev.GetY()+d, prev.GetX(), prev.GetY())
						m.c.Stroke()
					})
				} else {
					m.withState(func() {
						m.c.MoveTo(pt.GetX(), pt.GetY())
						m.c.CubicTo(pt.GetX()-d, pt.GetY()-d, prev.GetX()+d, prev.GetY()+d, prev.GetX(), prev.GetY())
						m.c.Stroke()
					})
				}
			}
		})
	})
}

func (m *Image) renderFakePath(points []Point) {
	d := m.CubicDelta

	gradColors := make([]color.RGBA, 0)
	for i := 0; i < len(points); i++ {
		gradColors = append(gradColors, m.getLineColor())
		gradColors = append(gradColors, m.getLineColor())
	}

	if m.renderHelpImg {
		return
	}

	m.withState(func() {

		// Draw the whole line
		m.withState(func() {
			startColor := gradColors[0]
			for i := 1; i < len(points); i++ {
				prev := points[i-1]
				pt := points[i]
				midColor := gradColors[(i*2)-1]
				lastColor := gradColors[i*2]
				grad := gg.NewLinearGradient(prev.GetX(), prev.GetY(), pt.GetX(), pt.GetY())
				grad.AddColorStop(0, startColor)
				grad.AddColorStop(0.5, midColor)
				grad.AddColorStop(1, lastColor)
				startColor = lastColor
				m.c.SetStrokeStyle(grad)
				m.withState(func() {
					m.c.SetLineWidth(1)
					m.c.MoveTo(prev.GetX(), prev.GetY())
					m.c.CubicTo(prev.GetX()-d, prev.GetY()-d, pt.GetX()+d, pt.GetY()+d, pt.GetX(), pt.GetY())
					m.c.Stroke()
				})
			}
		})

		// Semi transparent black line on top of the line
		m.withState(func() {
			m.c.SetLineWidth(4)
			m.c.SetColor(color.RGBA{R: 15, G: 15, B: 15, A: 100})
			for i := 1; i < len(points); i++ {
				prev := points[i-1]
				pt := points[i]
				m.c.MoveTo(prev.GetX(), prev.GetY())
				m.c.CubicTo(prev.GetX()-d, prev.GetY()-d, pt.GetX()+d, pt.GetY()+d, pt.GetX(), pt.GetY())
				m.c.Stroke()
				m.c.MoveTo(pt.GetX(), pt.GetY())
				m.c.CubicTo(pt.GetX()+d, pt.GetY()+d, prev.GetX()-d, prev.GetY()-d, prev.GetX(), prev.GetY())
				m.c.Stroke()
			}
		})

		// Draw the whole line again with dashes
		m.withState(func() {
			m.c.SetDash(5, 3)
			startColor := gradColors[0]
			for i := 1; i < len(points); i++ {
				prev := points[i-1]
				pt := points[i]

				midColor := gradColors[(i*2)-1]
				lastColor := gradColors[i*2]
				grad := gg.NewLinearGradient(prev.GetX(), prev.GetY(), pt.GetX(), pt.GetY())
				grad.AddColorStop(0, startColor)
				grad.AddColorStop(0.5, midColor)
				grad.AddColorStop(1, lastColor)
				startColor = lastColor
				m.c.SetStrokeStyle(grad)
				//m.c.SetColor(color.White)

				m.withState(func() {
					m.c.SetLineWidth(4)
					m.c.SetColor(color.RGBA{R: 15, G: 15, B: 15, A: 255})
					m.c.MoveTo(prev.GetX(), prev.GetY())
					m.c.CubicTo(prev.GetX()-d, prev.GetY()-d, pt.GetX()+d, pt.GetY()+d, pt.GetX(), pt.GetY())
					m.c.Stroke()
				})

				m.withState(func() {
					m.c.SetLineWidth(1.5)
					m.c.MoveTo(prev.GetX(), prev.GetY())
					m.c.CubicTo(prev.GetX()-d, prev.GetY()-d, pt.GetX()+d, pt.GetY()+d, pt.GetX(), pt.GetY())
					m.c.Stroke()
				})
			}
		})

		// Draw line edges with longer dashes
		m.withState(func() {
			m.c.SetDash(30, 200)
			m.c.SetLineWidth(1.5)
			startColor := gradColors[0]
			for i := 1; i < len(points); i++ {
				prev := points[i-1]
				pt := points[i]

				midColor := gradColors[(i*2)-1]
				lastColor := gradColors[i*2]
				grad := gg.NewLinearGradient(prev.GetX(), prev.GetY(), pt.GetX(), pt.GetY())
				grad.AddColorStop(0, startColor)
				grad.AddColorStop(0.5, midColor)
				grad.AddColorStop(1, lastColor)
				startColor = lastColor
				m.c.SetStrokeStyle(grad)

				m.c.MoveTo(prev.GetX(), prev.GetY())
				m.c.CubicTo(prev.GetX()-d, prev.GetY()-d, pt.GetX()+d, pt.GetY()+d, pt.GetX(), pt.GetY())
				m.c.Stroke()
				m.c.MoveTo(pt.GetX(), pt.GetY())
				m.c.CubicTo(pt.GetX()+d, pt.GetY()+d, prev.GetX()-d, prev.GetY()-d, prev.GetX(), prev.GetY())
				m.c.Stroke()
			}
		})
	})
}

func (m *Image) renderBackground() {
	m.withState(func() {
		//grad := gg.NewLinearGradient(0, 0, float64(m.imageWidth), float64(m.imageHeight))
		//grad.AddColorStop(0, color.RGBA{10, 10, 10, 255})
		//grad.AddColorStop(1, color.RGBA{50, 50, 50, 255})
		//m.c.SetFillStyle(grad)
		m.c.SetColor(color.RGBA{R: 15, G: 15, B: 15, A: 255})
		m.c.DrawRectangle(0, 0, float64(m.imageWidth), float64(m.imageHeight))
		m.c.Fill()
	})
}

func (m *Image) renderBorder() {
	m.withState(func() {
		m.c.SetColor(color.RGBA{R: 15, G: 15, B: 15, A: 255})
		m.c.DrawRectangle(0, 0, float64(m.imageWidth)-0.5, float64(m.imageHeight)-0.5)
		m.c.Stroke()
	})
}

func (m *Image) renderSquaresPattern() {
	y := 0.0
	for y < float64(m.imageHeight) {
		x := 0.0
		for x < float64(m.imageWidth) {
			num := uint8(m.RandInt(20, 40))
			m.c.SetColor(color.RGBA{R: num, G: num, B: num, A: 255})
			m.c.DrawRectangle(x, y, 10, 10)
			m.c.Fill()
			x += 11
		}
		y += 11
	}
}

func (m *Image) renderTrianglesPattern() {
	xStartRandOffset := m.RandFloat(0, 22)
	yStartRandOffset := m.RandFloat(0, 3)
	y := -yStartRandOffset
	for i := 0; i < 8; i++ {
		x := -11 - xStartRandOffset
		if i%2 == 0 {
			x -= 11
		}
		for j := 0; j < 13; j++ {
			m.withState(func() {
				m.c.Translate(x, y)
				num := uint8(m.RandInt(20, 40))
				m.c.SetColor(color.RGBA{R: num, G: num, B: num, A: 255})
				m.c.NewSubPath()
				m.c.MoveTo(0, 0)
				m.c.LineTo(10, 17)
				m.c.LineTo(20, 0)
				m.c.ClosePath()
				m.c.Fill()

				m.c.NewSubPath()
				m.c.MoveTo(11, 17)
				m.c.LineTo(21, 0)
				m.c.LineTo(31, 17)
				m.c.ClosePath()
				m.c.Fill()
			})
			x += 22
		}
		y += 18
	}
}

func (m *Image) renderDebugGrid() {
	if !m.displayDebug {
		return
	}
	m.withState(func() {
		m.c.SetLineWidth(1)
		m.c.SetColor(color.RGBA{R: 100, G: 100, B: 100, A: 255})
		for col := 0; col <= m.nbNumCols; col++ {
			x := float64(m.borderLeft+col*(m.numWidth+m.digitSpacing)) - float64(m.numHeight) - 0.5
			m.c.MoveTo(x, 0)
			m.c.LineTo(x, float64(m.imageHeight))
		}
		for row := 0; row <= m.nbNumRows; row++ {
			y := float64(m.borderTop+row*(m.numHeight+m.digitVertSpacing)) - 6 - 0.5
			m.c.MoveTo(0, y)
			m.c.LineTo(float64(m.imageWidth), y)
		}
		m.c.Stroke()
	})
}

func (m *Image) renderDigits(idx int64, clr color.RGBA) {
	m.withState(func() {
		x := m.borderLeft
		y := m.borderTop
		for row := 0; row < m.nbNumRows; row++ {
			for col := 0; col < m.nbNumCols; col++ {
				m.withState(func() {
					num := m.numbersMatrix[row][col]
					if idx == 0 {
						clr.A = num.Opacity
					}
					m.c.SetColor(clr)
					m.c.Translate(float64(x+m.numWidth/2), float64(y+m.numHeight/2))
					m.c.Rotate(gg.Radians(num.Angle))
					m.c.SetFontFace(num.Face)
					m.c.DrawString(strconv.Itoa(num.Num), float64(-m.numWidth/2), float64(m.numHeight/2))
					//m.withState(func() {
					//	m.c.SetColor(color.RGBA{255, 0, 0, 255})
					//	m.c.DrawCircle(0, 0, 2)
					//	m.c.Fill()
					//})
				})
				x += m.numWidth + m.digitSpacing
			}
			x = m.borderLeft
			y += m.numHeight + m.digitVertSpacing
		}
	})
}

func (m *Image) renderSignature() {
	m.withState(func() {
		m.c.SetFontFace(signatureFace)
		w, h := m.c.MeasureString("n0tr1v")
		num := uint8(50)
		m.c.SetColor(color.RGBA{R: num, G: num, B: num, A: 255})
		m.c.Translate(float64(m.imageWidth)/2, float64(m.imageHeight)/2)
		m.c.Rotate(-math.Pi / 2)
		m.c.Translate(-float64(m.numHeight/2)-w-15, float64(m.imageWidth)/2-h-1)
		m.c.DrawString("n0tr1v", float64(-m.numWidth/2), float64(m.numHeight/2))
	})
}

// encodedPNG encodes an image to PNG and returns
// the result as a byte slice.
func (m *Image) encodedPNG() []byte {
	var buf bytes.Buffer
	if err := png.Encode(&buf, m.RGBA); err != nil {
		panic(err.Error())
	}
	return buf.Bytes()
}

// WriteTo writes captcha image in PNG format into the given writer.
func (m *Image) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write(m.encodedPNG())
	return int64(n), err
}
