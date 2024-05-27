package color

// NoColor ...
var NoColor = false

// Terminal styling constants
const (
	knrm = "\x1B[0m"
	kred = "\x1B[31m"
	kgrn = "\x1B[32m"
	kyel = "\x1B[33m"
	kblu = "\x1B[34m"
	kmag = "\x1B[35m"
	kcyn = "\x1B[36m"
	kwht = "\x1B[37m"
)

func colorStr(color string, val string) string {
	if NoColor {
		return val
	}
	return color + val + knrm
}

// White ...
func White(val string) string {
	return colorStr(kwht, val)
}

// Cyan ...
func Cyan(val string) string {
	return colorStr(kcyn, val)
}

// Red ...
func Red(val string) string {
	return colorStr(kred, val)
}

// Blue ...
func Blue(val string) string {
	return colorStr(kblu, val)
}

// Yellow ...
func Yellow(val string) string {
	return colorStr(kyel, val)
}

// Green ...
func Green(val string) string {
	return colorStr(kgrn, val)
}

// Magenta ...
func Magenta(val string) string {
	return colorStr(kmag, val)
}
