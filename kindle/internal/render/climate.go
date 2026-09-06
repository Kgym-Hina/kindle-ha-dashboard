package render

import (
	"image"
	"image/color"
	"strings"

	"github.com/kgym-hina/kindle-ha-dashboard/kindle/internal/model"
)

func (r *Renderer) drawClimate(canvas *image.Gray, frame image.Rectangle, element model.Element, states map[string]model.EntityState) {
	style := element.Style
	fillColor := parseColor(style.Fill, color.Gray{Y: 247})
	strokeColor := parseColor(style.Stroke, color.Gray{Y: 175})
	radius := maxInt(int(style.Radius), 16)
	fillRounded(canvas, frame, fillColor, radius)
	strokeRounded(canvas, frame, strokeColor, radius, maxInt(int(style.BorderWidth), 1))

	title := strings.TrimSpace(element.Text)
	current, target, mode := "—", "—", ""
	var attributes map[string]any
	if element.Binding != nil {
		if state, ok := states[element.Binding.EntityID]; ok {
			attributes = state.Attributes
			if title == "" {
				title = attributeString(state.Attributes, "friendly_name", "")
			}
			current = attributeString(state.Attributes, "current_temperature", current)
			target = attributeString(state.Attributes, "temperature", target)
			mode = model.ClimateCurrentMode(state)
		}
	}
	if title == "" {
		title = "温控"
	}
	modes := model.ClimateModes(attributes)
	layout := model.ClimateHitLayout(element.Frame, modes)
	powerBox := rectFromFrame(layout.Power, canvas.Bounds())
	titleBox := image.Rect(frame.Min.X+12, frame.Min.Y+7, maxInt(frame.Min.X+13, powerBox.Min.X-8), powerBox.Max.Y)
	titleColor := style.Color
	if strings.TrimSpace(titleColor) == "" {
		titleColor = "#111111"
	}
	titleSize := style.FontSize
	if titleSize <= 0 {
		titleSize = 18
	}
	r.drawTextBox(canvas, title, titleBox, model.Style{Color: titleColor, FontSize: titleSize, FontWeight: "bold", Align: "left"})

	isOff := strings.EqualFold(mode, "off")
	powerFill, powerText := "#222222", "#ffffff"
	powerLabel := "关机"
	if isOff || mode == "" {
		powerFill, powerText = "#ffffff", "#222222"
		powerLabel = "开机"
	}
	fillRounded(canvas, powerBox, parseColor(powerFill, color.White), powerBox.Dy()/2)
	strokeRounded(canvas, powerBox, color.Gray{Y: 70}, powerBox.Dy()/2, 1)
	fillCircle(canvas, powerBox.Min.X+14, powerBox.Min.Y+powerBox.Dy()/2, 4, parseColor(powerText, color.Black))
	r.drawTextBox(canvas, powerLabel, image.Rect(powerBox.Min.X+23, powerBox.Min.Y, powerBox.Max.X-5, powerBox.Max.Y), model.Style{Color: powerText, FontSize: 12, Align: "center"})

	readingBox := rectFromFrame(layout.Reading, canvas.Bounds())
	middleX := readingBox.Min.X + readingBox.Dx()/2
	currentBox := image.Rect(readingBox.Min.X, readingBox.Min.Y, middleX-8, readingBox.Max.Y)
	targetBox := image.Rect(middleX+8, readingBox.Min.Y, readingBox.Max.X, readingBox.Max.Y)
	r.drawClimateMetric(canvas, currentBox, "室温", temperatureLabel(current), "#1b1b1b", 30)
	r.drawClimateMetric(canvas, targetBox, "目标温度", temperatureLabel(target), "#454545", 24)

	decreaseBox := rectFromFrame(layout.Decrease, canvas.Bounds())
	increaseBox := rectFromFrame(layout.Increase, canvas.Bounds())
	controlTextColor := "#222222"
	r.drawClimateButton(canvas, decreaseBox, "−", controlTextColor, 22)
	r.drawClimateButton(canvas, increaseBox, "+", controlTextColor, 22)
	targetControlBox := image.Rect(decreaseBox.Max.X+4, decreaseBox.Min.Y, increaseBox.Min.X-4, increaseBox.Max.Y)
	fillRounded(canvas, targetControlBox, color.Gray{Y: 232}, targetControlBox.Dy()/2)
	r.drawTextBox(canvas, "调温", targetControlBox, model.Style{Color: "#555555", FontSize: 13, Align: "center"})

	modeFontSize := 13
	if len(layout.ModeItems) >= 5 {
		modeFontSize = 11
	}
	for _, item := range layout.ModeItems {
		modeBox := rectFromFrame(item.Frame, canvas.Bounds())
		active := strings.EqualFold(item.Mode, mode)
		fill, text := "#ffffff", "#333333"
		if active {
			fill, text = "#222222", "#ffffff"
		}
		fillRounded(canvas, modeBox, parseColor(fill, color.White), modeBox.Dy()/2)
		strokeRounded(canvas, modeBox, color.Gray{Y: 95}, modeBox.Dy()/2, 1)
		r.drawTextBox(canvas, climateModeLabel(item.Mode), modeBox, model.Style{Color: text, FontSize: float64(modeFontSize), Align: "center"})
	}
}

func (r *Renderer) drawClimateMetric(canvas *image.Gray, frame image.Rectangle, label, value, valueColor string, valueSize float64) {
	labelHeight := minInt(18, maxInt(frame.Dy()/3, 12))
	valueFrame := image.Rect(frame.Min.X, frame.Min.Y, frame.Max.X, maxInt(frame.Min.Y+1, frame.Max.Y-labelHeight))
	labelFrame := image.Rect(frame.Min.X, valueFrame.Max.Y, frame.Max.X, frame.Max.Y)
	r.drawTextBox(canvas, value, valueFrame, model.Style{Color: valueColor, FontSize: valueSize, FontWeight: "bold", Align: "left"})
	r.drawTextBox(canvas, label, labelFrame, model.Style{Color: "#666666", FontSize: 11, Align: "left"})
}

func (r *Renderer) drawClimateButton(canvas *image.Gray, frame image.Rectangle, label, textColor string, fontSize float64) {
	fillRounded(canvas, frame, color.White, frame.Dy()/2)
	strokeRounded(canvas, frame, color.Gray{Y: 100}, frame.Dy()/2, 1)
	r.drawTextBox(canvas, label, frame, model.Style{Color: textColor, FontSize: fontSize, FontWeight: "bold", Align: "center"})
}
