package audit

import (
	"math"
	"strconv"
	"strings"
)

type cssColor struct {
	red   float64
	green float64
	blue  float64
	alpha float64
}

var basicNamedColors = map[string]string{
	"aqua":    "#00ffff",
	"black":   "#000000",
	"blue":    "#0000ff",
	"fuchsia": "#ff00ff",
	"gray":    "#808080",
	"green":   "#008000",
	"grey":    "#808080",
	"lime":    "#00ff00",
	"maroon":  "#800000",
	"navy":    "#000080",
	"olive":   "#808000",
	"orange":  "#ffa500",
	"purple":  "#800080",
	"red":     "#ff0000",
	"silver":  "#c0c0c0",
	"teal":    "#008080",
	"white":   "#ffffff",
	"yellow":  "#ffff00",
}

func parseCSSColor(raw string) (cssColor, bool) {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" || strings.Contains(value, "var(") || strings.Contains(value, "calc(") {
		return cssColor{}, false
	}
	if value == "transparent" {
		return cssColor{alpha: 0}, true
	}
	if hexadecimal, found := basicNamedColors[value]; found {
		return parseHexColor(hexadecimal)
	}
	if strings.HasPrefix(value, "#") {
		return parseHexColor(value)
	}

	name, body, found := colorFunction(value)
	if !found {
		return cssColor{}, false
	}
	switch name {
	case "rgb", "rgba":
		return parseRGBColor(body)
	case "hsl", "hsla":
		return parseHSLColor(body)
	case "oklab":
		return parseOKLabColor(body, false)
	case "oklch":
		return parseOKLabColor(body, true)
	case "color":
		return parsePredefinedColor(body)
	default:
		return cssColor{}, false
	}
}

func colorFunction(value string) (string, string, bool) {
	open := strings.IndexByte(value, '(')
	if open <= 0 || !strings.HasSuffix(value, ")") || strings.Contains(value[open+1:len(value)-1], "(") {
		return "", "", false
	}
	return strings.TrimSpace(value[:open]), strings.TrimSpace(value[open+1 : len(value)-1]), true
}

func parseHexColor(value string) (cssColor, bool) {
	digits := strings.TrimPrefix(value, "#")
	switch len(digits) {
	case 3, 4:
		var expanded strings.Builder
		for _, digit := range digits {
			expanded.WriteRune(digit)
			expanded.WriteRune(digit)
		}
		digits = expanded.String()
	case 6, 8:
	default:
		return cssColor{}, false
	}
	channels := []float64{0, 0, 0, 1}
	for index := 0; index < len(digits)/2; index++ {
		parsed, err := strconv.ParseUint(digits[index*2:index*2+2], 16, 8)
		if err != nil {
			return cssColor{}, false
		}
		channels[index] = float64(parsed) / 255
	}
	return cssColor{red: channels[0], green: channels[1], blue: channels[2], alpha: channels[3]}, true
}

func parseRGBColor(body string) (cssColor, bool) {
	components, alpha, found := functionalComponents(body, true)
	if !found || len(components) != 3 {
		return cssColor{}, false
	}
	channels := make([]float64, 3)
	for index, component := range components {
		value, percent, found := numericComponent(component)
		if !found {
			return cssColor{}, false
		}
		if percent {
			value /= 100
		} else {
			value /= 255
		}
		if !inUnitRange(value) {
			return cssColor{}, false
		}
		channels[index] = value
	}
	return cssColor{red: channels[0], green: channels[1], blue: channels[2], alpha: alpha}, true
}

func parseHSLColor(body string) (cssColor, bool) {
	components, alpha, found := functionalComponents(body, true)
	if !found || len(components) != 3 {
		return cssColor{}, false
	}
	hue, found := parseAngle(components[0])
	if !found {
		return cssColor{}, false
	}
	saturation, saturationPercent, found := numericComponent(components[1])
	if !found || !saturationPercent {
		return cssColor{}, false
	}
	lightness, lightnessPercent, found := numericComponent(components[2])
	if !found || !lightnessPercent {
		return cssColor{}, false
	}
	saturation /= 100
	lightness /= 100
	if !inUnitRange(saturation) || !inUnitRange(lightness) {
		return cssColor{}, false
	}

	chroma := (1 - math.Abs(2*lightness-1)) * saturation
	sector := math.Mod(hue/60, 6)
	intermediate := chroma * (1 - math.Abs(math.Mod(sector, 2)-1))
	var red, green, blue float64
	switch int(math.Floor(sector)) {
	case 0:
		red, green = chroma, intermediate
	case 1:
		red, green = intermediate, chroma
	case 2:
		green, blue = chroma, intermediate
	case 3:
		green, blue = intermediate, chroma
	case 4:
		red, blue = intermediate, chroma
	default:
		red, blue = chroma, intermediate
	}
	match := lightness - chroma/2
	return cssColor{red: red + match, green: green + match, blue: blue + match, alpha: alpha}, true
}

func parseOKLabColor(body string, polar bool) (cssColor, bool) {
	components, alpha, found := functionalComponents(body, false)
	if !found || len(components) != 3 {
		return cssColor{}, false
	}
	lightness, lightnessPercent, found := numericComponent(components[0])
	if !found {
		return cssColor{}, false
	}
	if lightnessPercent {
		lightness /= 100
	}
	if !inUnitRange(lightness) {
		return cssColor{}, false
	}

	second, secondPercent, found := numericComponent(components[1])
	if !found {
		return cssColor{}, false
	}
	if secondPercent {
		second *= 0.004
	}
	third, thirdPercent, found := numericComponent(components[2])
	if !found && polar {
		third, found = parseAngle(components[2])
	}
	if !found {
		return cssColor{}, false
	}
	if thirdPercent {
		third *= 0.004
	}

	a, b := second, third
	if polar {
		angle := third * math.Pi / 180
		a, b = second*math.Cos(angle), second*math.Sin(angle)
	}
	return oklabToSRGB(lightness, a, b, alpha)
}

func oklabToSRGB(lightness, a, b, alpha float64) (cssColor, bool) {
	long := lightness + 0.3963377773761749*a + 0.2158037573099136*b
	medium := lightness - 0.1055613458156586*a - 0.0638541728258133*b
	short := lightness - 0.0894841775298119*a - 1.2914855480194092*b
	long, medium, short = long*long*long, medium*medium*medium, short*short*short

	x := 1.2268798758459243*long - 0.5578149944602171*medium + 0.2813910456659647*short
	y := -0.0405757452148008*long + 1.1122868032803170*medium - 0.0717110580655164*short
	z := -0.0763729366746601*long - 0.4214933324022432*medium + 1.5869240198367816*short
	linearRed := (12831.0/3959.0)*x - (329.0/214.0)*y - (1974.0/3959.0)*z
	linearGreen := -(851781.0/878810.0)*x + (1648619.0/878810.0)*y + (36519.0/878810.0)*z
	linearBlue := (705.0/12673.0)*x - (2585.0/12673.0)*y + (705.0/667.0)*z
	if !inGamut(linearRed) || !inGamut(linearGreen) || !inGamut(linearBlue) {
		return cssColor{}, false
	}
	return cssColor{
		red: gammaEncode(clampUnit(linearRed)), green: gammaEncode(clampUnit(linearGreen)),
		blue: gammaEncode(clampUnit(linearBlue)), alpha: alpha,
	}, true
}

func parsePredefinedColor(body string) (cssColor, bool) {
	fields := strings.Fields(body)
	if len(fields) < 4 {
		return cssColor{}, false
	}
	space := fields[0]
	components, alpha, found := functionalComponents(strings.Join(fields[1:], " "), false)
	if !found || len(components) != 3 || (space != "srgb" && space != "srgb-linear") {
		return cssColor{}, false
	}
	channels := make([]float64, 3)
	for index, component := range components {
		value, percent, found := numericComponent(component)
		if !found {
			return cssColor{}, false
		}
		if percent {
			value /= 100
		}
		if !inUnitRange(value) {
			return cssColor{}, false
		}
		if space == "srgb-linear" {
			value = gammaEncode(value)
		}
		channels[index] = value
	}
	return cssColor{red: channels[0], green: channels[1], blue: channels[2], alpha: alpha}, true
}

func functionalComponents(body string, allowLegacyAlpha bool) ([]string, float64, bool) {
	if strings.Count(body, "/") > 1 {
		return nil, 0, false
	}
	main, alphaText, hasAlpha := strings.Cut(body, "/")
	main = strings.ReplaceAll(main, ",", " ")
	components := strings.Fields(main)
	if !hasAlpha && allowLegacyAlpha && len(components) == 4 {
		alphaText = components[3]
		components = components[:3]
		hasAlpha = true
	}
	alpha := 1.0
	if hasAlpha {
		fields := strings.Fields(strings.ReplaceAll(alphaText, ",", " "))
		if len(fields) != 1 {
			return nil, 0, false
		}
		value, percent, found := numericComponent(fields[0])
		if !found {
			return nil, 0, false
		}
		if percent {
			value /= 100
		}
		if !inUnitRange(value) {
			return nil, 0, false
		}
		alpha = value
	}
	return components, alpha, true
}

func numericComponent(raw string) (float64, bool, bool) {
	percent := strings.HasSuffix(raw, "%")
	if percent {
		raw = strings.TrimSuffix(raw, "%")
	}
	value, err := strconv.ParseFloat(raw, 64)
	return value, percent, err == nil && !math.IsNaN(value) && !math.IsInf(value, 0)
}

func parseAngle(raw string) (float64, bool) {
	multiplier := 1.0
	for suffix, factor := range map[string]float64{
		"deg": 1, "grad": 0.9, "rad": 180 / math.Pi, "turn": 360,
	} {
		if strings.HasSuffix(raw, suffix) {
			raw = strings.TrimSuffix(raw, suffix)
			multiplier = factor
			break
		}
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, false
	}
	value = math.Mod(value*multiplier, 360)
	if value < 0 {
		value += 360
	}
	return value, true
}

func contrastRatio(foreground, background cssColor) (float64, bool) {
	if background.alpha < 1 || foreground.alpha <= 0 {
		return 0, false
	}
	if foreground.alpha < 1 {
		foreground.red = foreground.red*foreground.alpha + background.red*(1-foreground.alpha)
		foreground.green = foreground.green*foreground.alpha + background.green*(1-foreground.alpha)
		foreground.blue = foreground.blue*foreground.alpha + background.blue*(1-foreground.alpha)
		foreground.alpha = 1
	}
	foregroundLuminance := relativeLuminance(foreground)
	backgroundLuminance := relativeLuminance(background)
	lighter, darker := foregroundLuminance, backgroundLuminance
	if lighter < darker {
		lighter, darker = darker, lighter
	}
	return (lighter + 0.05) / (darker + 0.05), true
}

func relativeLuminance(color cssColor) float64 {
	return 0.2126*linearize(color.red) + 0.7152*linearize(color.green) + 0.0722*linearize(color.blue)
}

func linearize(channel float64) float64 {
	if channel <= 0.04045 {
		return channel / 12.92
	}
	return math.Pow((channel+0.055)/1.055, 2.4)
}

func gammaEncode(channel float64) float64 {
	if channel <= 0.0031308 {
		return 12.92 * channel
	}
	return 1.055*math.Pow(channel, 1/2.4) - 0.055
}

func inUnitRange(value float64) bool {
	return value >= 0 && value <= 1
}

func inGamut(value float64) bool {
	const tolerance = 1e-7
	return value >= -tolerance && value <= 1+tolerance
}

func clampUnit(value float64) float64 {
	return math.Max(0, math.Min(1, value))
}
