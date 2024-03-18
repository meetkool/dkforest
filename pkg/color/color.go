package color

// NoColor must be set at compile time, not at runtime.
const NoColor = false

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

	kbold = "\x1B[1m"
	kfnt  = "\x1B[2m"
	kitl  = "\x1B[3m"
	kund  = "\x1B[4m"
	kblnk = "\x1B[5m"
	krvs  = "\x1B[7m"
	kcncl = "\x1B[8m"
)

func colorStr(color string, val string) string {
	if NoColor {
		return val
	}
	return color + val + knrm
}

// Reset sets the console color to the default.
func Reset(val string) string {
	return colorStr(knrm, val
