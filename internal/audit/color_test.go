package audit

import (
	"math"
	"testing"
)

func TestParseCSSColor(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		value string
		red   float64
		green float64
		blue  float64
		alpha float64
	}{
		{name: "short hexadecimal", value: "#f00", red: 1, alpha: 1},
		{name: "hexadecimal alpha", value: "#00ff0080", green: 1, alpha: 128.0 / 255.0},
		{name: "named", value: "navy", blue: 128.0 / 255.0, alpha: 1},
		{name: "modern rgb", value: "rgb(255 0 0 / 50%)", red: 1, alpha: 0.5},
		{name: "legacy rgba", value: "rgba(0, 255, 0, .25)", green: 1, alpha: 0.25},
		{name: "percentage rgb", value: "rgb(0% 0% 100%)", blue: 1, alpha: 1},
		{name: "hsl", value: "hsl(120deg 100% 50%)", green: 1, alpha: 1},
		{name: "hsl gradians", value: "hsl(100grad 100% 50%)", red: 0.5, green: 1, alpha: 1},
		{name: "oklch white", value: "oklch(100% 0 0)", red: 1, green: 1, blue: 1, alpha: 1},
		{name: "oklab black", value: "oklab(0 0 0)", alpha: 1},
		{name: "predefined srgb", value: "color(srgb 1 0 0 / 75%)", red: 1, alpha: 0.75},
		{name: "predefined linear", value: "color(srgb-linear 1 0 0)", red: 1, alpha: 1},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			color, found := parseCSSColor(testCase.value)
			if !found {
				t.Fatalf("parseCSSColor(%q) did not resolve", testCase.value)
			}
			for name, values := range map[string][2]float64{
				"red": {color.red, testCase.red}, "green": {color.green, testCase.green},
				"blue": {color.blue, testCase.blue}, "alpha": {color.alpha, testCase.alpha},
			} {
				if math.Abs(values[0]-values[1]) > 1e-6 {
					t.Errorf("%s = %.9f, want %.9f", name, values[0], values[1])
				}
			}
		})
	}
}

func TestParseCSSColorRejectsUnknownContext(t *testing.T) {
	t.Parallel()
	for _, value := range []string{
		"", "currentColor", "var(--brand)", "rgb(calc(1) 0 0)",
		"color(display-p3 1 0 0)", "oklch(70% 0.4 30)", "rgb(300 0 0)",
		"rgb(255, 0 0)", "rgb(255, 0, 0 / 50%)", "rgb(0 0 0 50%)",
		"oklab(50%, 0, 0)", "oklch(50% 0.1 25%)", "color(srgb 1, 0, 0)",
	} {
		if _, found := parseCSSColor(value); found {
			t.Errorf("parseCSSColor(%q) resolved an unsupported value", value)
		}
	}
}

func TestContrastRatio(t *testing.T) {
	t.Parallel()
	black, _ := parseCSSColor("#000")
	white, _ := parseCSSColor("#fff")
	ratio, resolved := contrastRatio(black, white)
	if !resolved || math.Abs(ratio-21) > 1e-12 {
		t.Fatalf("black on white ratio = %.12f, resolved %v; want 21", ratio, resolved)
	}

	halfBlack, _ := parseCSSColor("rgb(0 0 0 / 50%)")
	ratio, resolved = contrastRatio(halfBlack, white)
	if !resolved || math.Abs(ratio-3.976653) > 0.00001 {
		t.Errorf("half black on white ratio = %.9f, resolved %v; want about 3.976653", ratio, resolved)
	}
}

func TestContrastRatioRejectsUnknownCompositing(t *testing.T) {
	t.Parallel()
	transparent, _ := parseCSSColor("transparent")
	black, _ := parseCSSColor("black")
	if _, resolved := contrastRatio(black, transparent); resolved {
		t.Error("transparent background resolved without an ancestor colour")
	}
	if _, resolved := contrastRatio(transparent, black); resolved {
		t.Error("fully transparent foreground resolved as visible text")
	}
}
