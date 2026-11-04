package ui

import "math"

// Night-garden palette: deep indigo, lavender, hot pink, warm amber.
var (
	Background  = Hex("#07070e")
	PanelBG     = Hex("#0d0d18")
	PanelRaised = Hex("#131322")
	BorderColor = Hex("#23233a")
	Ink         = Hex("#e9e4ff")
	Muted       = Hex("#8a85a3")
	Dim         = Hex("#4e4a66")
	Accent      = Hex("#c9a7ff")
	Pink        = Hex("#ff86c2")
	Peach       = Hex("#ffc48a")
	Mint        = Hex("#7fe7d4")
	Blue        = Hex("#6d8bff")
	Danger      = Hex("#ff6b8a")
	Success     = Hex("#8fe7b0")
	Cream       = Hex("#fff3dd")
	Violet      = Hex("#7b5cff")
	Magenta     = Hex("#d568ff")
)

// warmGradient travels violet → magenta → pink → amber.
func warmGradient(t float64) RGB {
	t = clamp01(t)
	switch {
	case t < 0.4:
		return lerp(Violet, Magenta, t/0.4)
	case t < 0.75:
		return lerp(Magenta, Pink, (t-0.4)/0.35)
	default:
		return lerp(Pink, Peach, (t-0.75)/0.25)
	}
}

// auroraGradient travels teal → blue → violet → pink.
func auroraGradient(t float64) RGB {
	t = clamp01(t)
	switch {
	case t < 0.35:
		return lerp(Mint, Blue, t/0.35)
	case t < 0.7:
		return lerp(Blue, Violet, (t-0.35)/0.35)
	default:
		return lerp(Violet, Pink, (t-0.7)/0.3)
	}
}

func pulseColor(base RGB, phase float64, depth float64) RGB {
	amount := (math.Sin(phase) + 1) / 2 * depth
	return lerp(base, Cream, amount)
}
