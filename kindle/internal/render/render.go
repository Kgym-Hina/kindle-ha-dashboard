package render

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/kgym-hina/kindle-ha-dashboard/kindle/internal/model"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

type Dialog struct {
	Title     string
	Message   string
	ExpiresAt int64
}

const NavigationBarHeight = 72

const (
	maxImageBytes  = 8 << 20
	maxImagePixels = int64(2_000_000)
	maxImageWidth  = 4096
	maxImageHeight = 4096
)

type Navigation struct {
	Show         bool
	CanGoBack    bool
	IsSettings   bool
	PressedIndex int
}

type Renderer struct {
	HTTPClient   *http.Client
	HAURL        string
	Token        string
	FontPath     string
	FontBoldPath string

	regularFontOnce sync.Once
	regularFont     *opentype.Font
	boldFontOnce    sync.Once
	boldFont        *opentype.Font
}

func (r *Renderer) Render(document *model.Document, pageIndex int, dialog *Dialog, navigation Navigation, states map[string]model.EntityState) (*image.Gray, error) {
	if document == nil {
		return nil, errors.New("cannot render a nil document")
	}
	page := document.Page(pageIndex)
	width, height := document.Target.Width, document.Target.Height
	canvas := image.NewGray(image.Rect(0, 0, width, height))
	fill(canvas, parseColor(document.Target.Background, color.White))
	if page.Background != "" {
		fill(canvas, parseColor(page.Background, parseColor(document.Target.Background, color.White)))
	}
	for _, element := range page.Elements {
		if err := r.drawElement(canvas, element, states); err != nil {
			return nil, fmt.Errorf("render element %s: %w", element.ID, err)
		}
	}
	if navigation.Show {
		r.drawNavigation(canvas, navigation)
	}
	if dialog != nil {
		r.drawDialog(canvas, *dialog)
	}
	return canvas, nil
}

func (r *Renderer) drawElement(canvas *image.Gray, element model.Element, states map[string]model.EntityState) error {
	frame := rectFromFrame(element.Frame, canvas.Bounds())
	style := element.Style
	fillColor := parseColor(style.Fill, color.Transparent)
	strokeColor := parseColor(style.Stroke, parseColor(style.Color, color.Black))
	lineWidth := maxInt(int(style.BorderWidth), 1)
	switch element.Type {
	case "rect":
		if fillColor != color.Transparent {
			fillRounded(canvas, frame, fillColor, maxInt(int(style.Radius), 0))
		}
		if style.Stroke != "" || style.BorderWidth > 0 {
			strokeRounded(canvas, frame, strokeColor, maxInt(int(style.Radius), 0), lineWidth)
		}
	case "line":
		drawLine(canvas, frame, strokeColor, lineWidth)
	case "text":
		r.drawTextBox(canvas, resolveElementText(element, states), frame, style)
	case "button":
		if fillColor != color.Transparent {
			fillRounded(canvas, frame, fillColor, maxInt(int(style.Radius), 0))
		}
		strokeRounded(canvas, frame, strokeColor, maxInt(int(style.Radius), 0), lineWidth)
		r.drawTextBox(canvas, resolveElementText(element, states), frame, style)
	case "switch":
		r.drawSwitch(canvas, frame, element, states)
	case "climate":
		r.drawClimate(canvas, frame, element, states)
	case "image", "image_button":
		if element.Image == nil || strings.TrimSpace(element.Image.Src) == "" {
			if element.Type == "image" {
				r.drawImagePlaceholder(canvas, frame, "未设置图片")
			}
			return nil
		}
		asset, err := r.loadImage(element.Image.Src)
		if err != nil {
			if element.Type == "image" {
				r.drawImagePlaceholder(canvas, frame, "图片无法显示")
			}
			return nil
		}
		drawImage(canvas, frame, asset, element.Image.Fit)
	default:
		return fmt.Errorf("unsupported type %q", element.Type)
	}
	return nil
}

func (r *Renderer) drawTextBox(canvas *image.Gray, value string, frame image.Rectangle, style model.Style) {
	if strings.TrimSpace(value) == "" {
		return
	}
	fontSize := style.FontSize
	if fontSize <= 0 {
		fontSize = 16
	}
	face := r.face(fontSize, style.FontWeight == "bold")
	fontColor := parseColor(style.Color, color.Black)
	lineHeight := maxInt(int(math.Ceil(fontSize*1.15)), 1)
	metrics := face.Metrics()
	leading := maxInt(lineHeight-metrics.Ascent.Ceil()-metrics.Descent.Ceil(), 0)
	lines := wrapText(value, face, maxInt(frame.Dx()-8, 1))
	totalHeight := len(lines) * lineHeight
	y := frame.Min.Y + maxInt((frame.Dy()-totalHeight)/2, 0) + metrics.Ascent.Ceil() + leading/2
	for _, line := range lines {
		if y > frame.Max.Y {
			break
		}
		lineWidth := font.MeasureString(face, line).Ceil()
		x := frame.Min.X + 4
		switch style.Align {
		case "center":
			x = frame.Min.X + maxInt((frame.Dx()-lineWidth)/2, 0)
		case "right":
			x = frame.Max.X - lineWidth - 4
		}
		if x < frame.Min.X {
			x = frame.Min.X
		}
		drawer := &font.Drawer{Dst: canvas, Src: image.NewUniform(fontColor), Face: face, Dot: fixed.P(x, y)}
		drawer.DrawString(line)
		y += lineHeight
	}
}

func resolveElementText(element model.Element, states map[string]model.EntityState) string {
	if element.Binding == nil {
		return element.Text
	}
	value := resolveBindingValue(element.Binding, states)
	if strings.Contains(element.Text, "{value}") {
		return strings.ReplaceAll(element.Text, "{value}", value)
	}
	if strings.TrimSpace(element.Text) == "" || element.Text == "新文本" {
		return value
	}
	return element.Text + value
}

func resolveBindingValue(binding *model.Binding, states map[string]model.EntityState) string {
	if binding == nil {
		return ""
	}
	state, ok := states[binding.EntityID]
	if !ok {
		return "—"
	}
	value := state.State
	if binding.Field != "" && binding.Field != "state" {
		if attribute, exists := state.Attributes[binding.Field]; exists {
			value = formatAttribute(attribute)
		} else {
			value = "—"
		}
	}
	if binding.Decimals != nil {
		if parsed, err := strconv.ParseFloat(value, 64); err == nil {
			value = strconv.FormatFloat(parsed, 'f', *binding.Decimals, 64)
		}
	}
	return binding.Prefix + value + binding.Suffix
}

func formatAttribute(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(typed), 'f', -1, 32)
	case bool:
		if typed {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprint(typed)
	}
}

func (r *Renderer) drawSwitch(canvas *image.Gray, frame image.Rectangle, element model.Element, states map[string]model.EntityState) {
	style := element.Style
	fillColor := parseColor(style.Fill, color.White)
	strokeColor := parseColor(style.Stroke, color.Gray{Y: 190})
	radius := maxInt(int(style.Radius), 16)
	fillRounded(canvas, frame, fillColor, radius)
	strokeRounded(canvas, frame, strokeColor, radius, maxInt(int(style.BorderWidth), 1))
	status := "未绑定"
	isOn := false
	if element.Binding != nil {
		status = "关"
		if state, ok := states[element.Binding.EntityID]; ok && isOnState(state.State) {
			status = "开"
			isOn = true
		}
	}
	title := element.Text
	if strings.TrimSpace(title) == "" {
		title = "开关"
	}
	padding := minInt(18, maxInt(frame.Dx()/10, 8))
	trackHeight := minInt(40, maxInt(frame.Dy()-24, 28))
	trackWidth := minInt(84, maxInt(frame.Dx()/3, 64))
	track := image.Rect(frame.Max.X-padding-trackWidth, frame.Min.Y+(frame.Dy()-trackHeight)/2, frame.Max.X-padding, frame.Min.Y+(frame.Dy()-trackHeight)/2+trackHeight)
	trackColor := color.Gray{Y: 218}
	if isOn {
		trackColor = color.Gray{Y: 35}
	}
	fillRounded(canvas, track, trackColor, trackHeight/2)
	knobRadius := maxInt(trackHeight/2-4, 5)
	knobX := track.Min.X + knobRadius + 4
	if isOn {
		knobX = track.Max.X - knobRadius - 4
	}
	fillCircle(canvas, knobX, track.Min.Y+trackHeight/2, knobRadius, color.White)
	strokeCircle(canvas, knobX, track.Min.Y+trackHeight/2, knobRadius, color.Gray{Y: 130}, 1)
	labelFrame := image.Rect(frame.Min.X+padding, frame.Min.Y+8, track.Min.X-padding, frame.Max.Y-8)
	labelColor := style.Color
	if strings.TrimSpace(labelColor) == "" {
		labelColor = "#111111"
	}
	labelStyle := model.Style{Color: labelColor, FontSize: style.FontSize, FontWeight: style.FontWeight, Align: "left"}
	if labelStyle.FontSize <= 0 {
		labelStyle.FontSize = 18
	}
	labelHalf := maxInt(labelFrame.Dy()/2, 1)
	r.drawTextBox(canvas, title, image.Rect(labelFrame.Min.X, labelFrame.Min.Y, labelFrame.Max.X, labelFrame.Min.Y+labelHalf), labelStyle)
	if element.Binding == nil {
		status = "未绑定"
	} else if isOn {
		status = "已开启"
	} else {
		status = "已关闭"
	}
	r.drawTextBox(canvas, status, image.Rect(labelFrame.Min.X, labelFrame.Min.Y+labelHalf, labelFrame.Max.X, labelFrame.Max.Y), model.Style{Color: "#696969", FontSize: 13, Align: "left"})
}

func (r *Renderer) drawClimate(canvas *image.Gray, frame image.Rectangle, element model.Element, states map[string]model.EntityState) {
	style := element.Style
	fillColor := parseColor(style.Fill, color.Gray{Y: 247})
	strokeColor := parseColor(style.Stroke, color.Gray{Y: 175})
	radius := maxInt(int(style.Radius), 16)
	fillRounded(canvas, frame, fillColor, radius)
	strokeRounded(canvas, frame, strokeColor, radius, maxInt(int(style.BorderWidth), 1))
	title := element.Text
	current, target, mode := "—", "—", "—"
	if element.Binding != nil {
		if state, ok := states[element.Binding.EntityID]; ok {
			title = attributeString(state.Attributes, "friendly_name", title)
			current = attributeString(state.Attributes, "current_temperature", current)
			target = attributeString(state.Attributes, "temperature", target)
			mode = attributeString(state.Attributes, "hvac_mode", state.State)
		}
	}
	if strings.TrimSpace(title) == "" {
		title = "温控"
	}
	padding := minInt(18, maxInt(frame.Dx()/12, 8))
	titleColor := style.Color
	if strings.TrimSpace(titleColor) == "" {
		titleColor = "#111111"
	}
	titleStyle := model.Style{Color: titleColor, FontSize: 18, FontWeight: "bold", Align: "left"}
	r.drawTextBox(canvas, title, image.Rect(frame.Min.X+padding, frame.Min.Y+10, frame.Max.X-padding-120, frame.Min.Y+48), titleStyle)
	modeLabel := climateModeLabel(mode)
	modeWidth := minInt(112, maxInt(frame.Dx()/4, 76))
	modeBox := image.Rect(frame.Max.X-padding-modeWidth, frame.Min.Y+12, frame.Max.X-padding, frame.Min.Y+42)
	fillRounded(canvas, modeBox, color.Gray{Y: 225}, 15)
	r.drawTextBox(canvas, modeLabel, modeBox, model.Style{Color: "#4b4b4b", FontSize: 13, Align: "center"})
	readingTop := frame.Min.Y + 58
	dividerY := frame.Max.Y - 72
	if dividerY < readingTop+46 {
		dividerY = frame.Min.Y + (frame.Dy()*2)/3
	}
	currentBox := image.Rect(frame.Min.X+padding, readingTop, frame.Min.X+frame.Dx()/2, dividerY-4)
	targetBox := image.Rect(frame.Min.X+frame.Dx()/2, readingTop+12, frame.Max.X-padding, dividerY-4)
	r.drawTextBox(canvas, temperatureLabel(current), currentBox, model.Style{Color: "#202020", FontSize: 32, FontWeight: "bold", Align: "left"})
	r.drawTextBox(canvas, "当前温度", image.Rect(currentBox.Min.X, currentBox.Max.Y-26, currentBox.Max.X, currentBox.Max.Y), model.Style{Color: "#696969", FontSize: 12, Align: "left"})
	r.drawTextBox(canvas, temperatureLabel(target), targetBox, model.Style{Color: "#3f3f3f", FontSize: 22, FontWeight: "bold", Align: "left"})
	r.drawTextBox(canvas, "目标温度", image.Rect(targetBox.Min.X, targetBox.Max.Y-26, targetBox.Max.X, targetBox.Max.Y), model.Style{Color: "#696969", FontSize: 12, Align: "left"})
	drawLine(canvas, image.Rect(frame.Min.X+padding, dividerY, frame.Max.X-padding, dividerY+1), color.Gray{Y: 205}, 1)
	controlBox := image.Rect(frame.Min.X+padding, dividerY+5, frame.Max.X-padding, frame.Max.Y-5)
	third := maxInt(controlBox.Dx()/3, 1)
	r.drawTextBox(canvas, "−  调低", image.Rect(controlBox.Min.X, controlBox.Min.Y, controlBox.Min.X+third, controlBox.Max.Y), model.Style{Color: "#3d3d3d", FontSize: 14, Align: "center"})
	r.drawTextBox(canvas, "模式", image.Rect(controlBox.Min.X+third, controlBox.Min.Y, controlBox.Min.X+third*2, controlBox.Max.Y), model.Style{Color: "#696969", FontSize: 13, Align: "center"})
	r.drawTextBox(canvas, "＋  调高", image.Rect(controlBox.Min.X+third*2, controlBox.Min.Y, controlBox.Max.X, controlBox.Max.Y), model.Style{Color: "#3d3d3d", FontSize: 14, Align: "center"})
}

func climateModeLabel(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "off":
		return "关闭"
	case "heat":
		return "制热"
	case "cool":
		return "制冷"
	case "auto", "heat_cool":
		return "自动"
	case "dry":
		return "除湿"
	case "fan_only":
		return "送风"
	default:
		if strings.TrimSpace(value) == "" {
			return "未绑定"
		}
		return value
	}
}

func temperatureLabel(value string) string {
	if strings.TrimSpace(value) == "" || value == "—" {
		return "—"
	}
	return value + "°"
}

func attributeString(attributes map[string]any, key, fallback string) string {
	if value, ok := attributes[key]; ok {
		return formatAttribute(value)
	}
	return fallback
}

func isOnState(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "on", "true", "1", "yes":
		return true
	default:
		return false
	}
}

func (r *Renderer) drawNavigation(canvas *image.Gray, navigation Navigation) {
	barHeight := minInt(NavigationBarHeight, canvas.Bounds().Dy())
	bar := image.Rect(0, canvas.Bounds().Dy()-barHeight, canvas.Bounds().Dx(), canvas.Bounds().Dy())
	fillRect(canvas, bar, color.White)
	drawLine(canvas, image.Rect(bar.Min.X, bar.Min.Y, bar.Max.X, bar.Min.Y+2), color.Black, 2)
	itemWidth := maxInt(bar.Dx()/3, 1)
	labels := []string{"‹ 返回", "主页", "设置"}
	for index, label := range labels {
		left := bar.Min.X + index*itemWidth
		right := left + itemWidth
		if index == 2 {
			right = bar.Max.X
		}
		itemStyle := model.Style{Color: "#111111", FontSize: 16, Align: "center"}
		if index == navigation.PressedIndex {
			fillRect(canvas, image.Rect(left, bar.Min.Y+2, right, bar.Max.Y), color.Black)
			itemStyle.Color = "#ffffff"
		}
		if index == 0 && !navigation.CanGoBack {
			if index != navigation.PressedIndex {
				itemStyle.Color = "#999999"
			}
		}
		if index == 2 && navigation.IsSettings {
			itemStyle.FontWeight = "bold"
		}
		r.drawTextBox(canvas, label, image.Rect(left, bar.Min.Y+2, right, bar.Max.Y), itemStyle)
	}
}

func (r *Renderer) face(size float64, bold bool) font.Face {
	if size <= 0 {
		size = 16
	}
	parsed := r.loadFont(bold)
	if parsed != nil {
		if face, err := opentype.NewFace(parsed, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull}); err == nil {
			return face
		}
	}
	return basicfont.Face7x13
}

func (r *Renderer) loadFont(bold bool) *opentype.Font {
	if bold {
		r.boldFontOnce.Do(func() {
			r.boldFont = parseFontFile(r.FontBoldPath)
		})
		if r.boldFont != nil {
			return r.boldFont
		}
	}
	r.regularFontOnce.Do(func() {
		r.regularFont = parseFontFile(r.FontPath)
	})
	return r.regularFont
}

func parseFontFile(path string) *opentype.Font {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	parsed, err := opentype.Parse(data)
	if err != nil {
		return nil
	}
	return parsed
}

func (r Renderer) loadImage(source string) (image.Image, error) {
	source = strings.TrimSpace(source)
	if strings.HasPrefix(source, "data:") {
		comma := strings.IndexByte(source, ',')
		if comma < 0 {
			return nil, errors.New("invalid data image")
		}
		meta, payload := source[:comma], source[comma+1:]
		var data []byte
		var err error
		if strings.Contains(meta, ";base64") {
			if base64.StdEncoding.DecodedLen(len(payload)) > maxImageBytes {
				return nil, fmt.Errorf("image file exceeds %d MiB", maxImageBytes/(1<<20))
			}
			data, err = base64.StdEncoding.DecodeString(payload)
		} else {
			if len(payload) > maxImageBytes {
				return nil, fmt.Errorf("image file exceeds %d MiB", maxImageBytes/(1<<20))
			}
			decoded, decodeErr := url.PathUnescape(payload)
			data, err = []byte(decoded), decodeErr
		}
		if err != nil {
			return nil, fmt.Errorf("decode data image: %w", err)
		}
		return decodeImage(bytes.NewReader(data))
	}
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		client := r.HTTPClient
		if client == nil {
			client = &http.Client{Timeout: 10 * time.Second}
		}
		request, err := http.NewRequest(http.MethodGet, source, nil)
		if err != nil {
			return nil, err
		}
		if r.Token != "" && sameOrigin(r.HAURL, source) {
			request.Header.Set("Authorization", "Bearer "+r.Token)
		}
		response, err := client.Do(request)
		if err != nil {
			return nil, err
		}
		defer response.Body.Close()
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			return nil, fmt.Errorf("download image: %s", response.Status)
		}
		return decodeImage(response.Body)
	}
	file, err := os.Open(filepath.Clean(source))
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return decodeImage(file)
}

func decodeImage(reader io.Reader) (image.Image, error) {
	data, err := io.ReadAll(io.LimitReader(reader, maxImageBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxImageBytes {
		return nil, fmt.Errorf("image file exceeds %d MiB", maxImageBytes/(1<<20))
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if config.Width <= 0 || config.Height <= 0 || config.Width > maxImageWidth || config.Height > maxImageHeight || int64(config.Width)*int64(config.Height) > maxImagePixels {
		return nil, fmt.Errorf("image dimensions %dx%d are too large", config.Width, config.Height)
	}
	decoded, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if format == "" {
		return nil, errors.New("unknown image format")
	}
	return decoded, nil
}

func (r *Renderer) drawImagePlaceholder(canvas *image.Gray, frame image.Rectangle, message string) {
	fillRounded(canvas, frame, color.Gray{Y: 248}, 8)
	strokeRounded(canvas, frame, color.Gray{Y: 185}, 8, 1)
	r.drawTextBox(canvas, message, frame, model.Style{Color: "#777777", FontSize: 14, Align: "center"})
}

func wrapText(value string, face font.Face, maxWidth int) []string {
	var lines []string
	for _, paragraph := range strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n") {
		if paragraph == "" {
			lines = append(lines, "")
			continue
		}
		line := ""
		for _, token := range textTokens(paragraph) {
			if line == "" && strings.TrimSpace(token) == "" {
				continue
			}
			candidate := line + token
			if font.MeasureString(face, candidate).Ceil() <= maxWidth {
				line = candidate
				continue
			}
			lines = append(lines, strings.TrimRight(line, " \t"))
			line = strings.TrimLeft(token, " \t")
		}
		lines = append(lines, strings.TrimRight(line, " \t"))
	}
	return lines
}

func textTokens(value string) []string {
	var tokens []string
	var word []rune
	flushWord := func() {
		if len(word) > 0 {
			tokens = append(tokens, string(word))
			word = nil
		}
	}
	for _, char := range value {
		switch {
		case unicode.IsSpace(char):
			flushWord()
			tokens = append(tokens, string(char))
		case isCJKCharacter(char):
			flushWord()
			tokens = append(tokens, string(char))
		default:
			word = append(word, char)
		}
	}
	flushWord()
	return tokens
}

func isCJKCharacter(char rune) bool {
	return (char >= 0x2e80 && char <= 0x2fff) ||
		(char >= 0x3000 && char <= 0x30ff) ||
		(char >= 0x3400 && char <= 0x4dbf) ||
		(char >= 0x4e00 && char <= 0x9fff) ||
		(char >= 0xac00 && char <= 0xd7af) ||
		(char >= 0xf900 && char <= 0xfaff) ||
		(char >= 0xff00 && char <= 0xffef)
}

func drawImage(canvas *image.Gray, frame image.Rectangle, source image.Image, fit string) {
	bounds := source.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		return
	}
	destination := frame
	if fit != "stretch" {
		scaleX := float64(frame.Dx()) / float64(bounds.Dx())
		scaleY := float64(frame.Dy()) / float64(bounds.Dy())
		scale := scaleX
		if fit == "cover" {
			scale = maxFloat(scaleX, scaleY)
		} else {
			scale = minFloat(scaleX, scaleY)
		}
		width := maxInt(int(float64(bounds.Dx())*scale), 1)
		height := maxInt(int(float64(bounds.Dy())*scale), 1)
		destination = image.Rect(
			frame.Min.X+(frame.Dx()-width)/2,
			frame.Min.Y+(frame.Dy()-height)/2,
			frame.Min.X+(frame.Dx()-width)/2+width,
			frame.Min.Y+(frame.Dy()-height)/2+height,
		)
	}
	resized := image.NewGray(destination)
	xdraw.ApproxBiLinear.Scale(resized, destination, source, bounds, xdraw.Over, nil)
	for y := destination.Min.Y; y < destination.Max.Y; y++ {
		for x := destination.Min.X; x < destination.Max.X; x++ {
			if image.Pt(x, y).In(canvas.Bounds()) {
				canvas.SetGray(x, y, resized.GrayAt(x, y))
			}
		}
	}
}

func (r *Renderer) drawDialog(canvas *image.Gray, dialog Dialog) {
	bounds := canvas.Bounds()
	margin := 34
	box := image.Rect(margin, bounds.Dy()/2-150, bounds.Dx()-margin, bounds.Dy()/2+150)
	fillRounded(canvas, box, color.White, 14)
	strokeRounded(canvas, box, color.Black, 14, 3)
	titleFace := r.face(22, true)
	messageFace := r.face(16, false)
	drawer := &font.Drawer{Dst: canvas, Src: image.NewUniform(color.Black), Face: titleFace}
	title := dialog.Title
	if title == "" {
		title = "Message"
	}
	drawer.Dot = fixed.P(box.Min.X+24, box.Min.Y+48)
	drawer.DrawString(title)
	lineHeight := maxInt(int(math.Ceil(16*1.15)), 1)
	for index, line := range wrapText(dialog.Message, messageFace, box.Dx()-48) {
		drawer.Face = messageFace
		drawer.Dot = fixed.P(box.Min.X+24, box.Min.Y+88+index*lineHeight)
		drawer.DrawString(line)
	}
	drawer.Face = messageFace
	drawer.Dot = fixed.P(box.Max.X-96, box.Max.Y-28)
	drawer.DrawString("Tap to close")
}

func rectFromFrame(frame model.Frame, bounds image.Rectangle) image.Rectangle {
	x := int(frame.X)
	y := int(frame.Y)
	width := maxInt(int(frame.Width), 1)
	height := maxInt(int(frame.Height), 1)
	return image.Rect(x, y, x+width, y+height).Intersect(bounds)
}

func fill(canvas *image.Gray, value color.Color) {
	fillRect(canvas, canvas.Bounds(), value)
}

func fillRect(canvas *image.Gray, rect image.Rectangle, value color.Color) {
	rect = rect.Intersect(canvas.Bounds())
	gray := color.GrayModel.Convert(value).(color.Gray)
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			canvas.SetGray(x, y, gray)
		}
	}
}

func fillRounded(canvas *image.Gray, rect image.Rectangle, value color.Color, radius int) {
	rect = rect.Intersect(canvas.Bounds())
	if rect.Empty() {
		return
	}
	gray := color.GrayModel.Convert(value).(color.Gray)
	if radius <= 0 {
		for y := rect.Min.Y; y < rect.Max.Y; y++ {
			for x := rect.Min.X; x < rect.Max.X; x++ {
				canvas.SetGray(x, y, gray)
			}
		}
		return
	}
	radius = minInt(radius, minInt(rect.Dx(), rect.Dy())/2)
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			if roundedContains(rect, x, y, radius) {
				canvas.SetGray(x, y, gray)
			}
		}
	}
}

func fillCircle(canvas *image.Gray, centerX, centerY, radius int, value color.Color) {
	if radius <= 0 {
		return
	}
	gray := color.GrayModel.Convert(value).(color.Gray)
	radiusSquared := radius * radius
	for y := centerY - radius; y <= centerY+radius; y++ {
		for x := centerX - radius; x <= centerX+radius; x++ {
			if !image.Pt(x, y).In(canvas.Bounds()) {
				continue
			}
			deltaX, deltaY := x-centerX, y-centerY
			if deltaX*deltaX+deltaY*deltaY <= radiusSquared {
				canvas.SetGray(x, y, gray)
			}
		}
	}
}

func strokeCircle(canvas *image.Gray, centerX, centerY, radius int, value color.Color, width int) {
	if radius <= 0 || width <= 0 {
		return
	}
	gray := color.GrayModel.Convert(value).(color.Gray)
	outerSquared := radius * radius
	innerRadius := maxInt(radius-width, 0)
	innerSquared := innerRadius * innerRadius
	for y := centerY - radius; y <= centerY+radius; y++ {
		for x := centerX - radius; x <= centerX+radius; x++ {
			if !image.Pt(x, y).In(canvas.Bounds()) {
				continue
			}
			deltaX, deltaY := x-centerX, y-centerY
			distanceSquared := deltaX*deltaX + deltaY*deltaY
			if distanceSquared <= outerSquared && distanceSquared > innerSquared {
				canvas.SetGray(x, y, gray)
			}
		}
	}
}

func strokeRounded(canvas *image.Gray, rect image.Rectangle, value color.Color, radius, width int) {
	rect = rect.Intersect(canvas.Bounds())
	if rect.Empty() {
		return
	}
	gray := color.GrayModel.Convert(value).(color.Gray)
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			outer := roundedContains(rect, x, y, radius)
			innerRect := image.Rect(rect.Min.X+width, rect.Min.Y+width, rect.Max.X-width, rect.Max.Y-width)
			inner := innerRect.Dx() > 0 && innerRect.Dy() > 0 && roundedContains(innerRect, x, y, maxInt(radius-width, 0))
			if outer && !inner {
				canvas.SetGray(x, y, gray)
			}
		}
	}
}

func roundedContains(rect image.Rectangle, x, y, radius int) bool {
	if radius <= 0 {
		return image.Pt(x, y).In(rect)
	}
	if !image.Pt(x, y).In(rect) {
		return false
	}
	if x >= rect.Min.X+radius && x < rect.Max.X-radius {
		return true
	}
	if y >= rect.Min.Y+radius && y < rect.Max.Y-radius {
		return true
	}
	centers := [][2]int{
		{rect.Min.X + radius, rect.Min.Y + radius},
		{rect.Max.X - radius - 1, rect.Min.Y + radius},
		{rect.Min.X + radius, rect.Max.Y - radius - 1},
		{rect.Max.X - radius - 1, rect.Max.Y - radius - 1},
	}
	for _, center := range centers {
		dx, dy := x-center[0], y-center[1]
		if dx*dx+dy*dy <= radius*radius {
			return true
		}
	}
	return false
}

func drawLine(canvas *image.Gray, rect image.Rectangle, value color.Color, width int) {
	rect = rect.Intersect(canvas.Bounds())
	if rect.Empty() {
		return
	}
	gray := color.GrayModel.Convert(value).(color.Gray)
	if rect.Dx() >= rect.Dy() {
		for y := rect.Min.Y; y < rect.Min.Y+width && y < rect.Max.Y; y++ {
			for x := rect.Min.X; x < rect.Max.X; x++ {
				canvas.SetGray(x, y, gray)
			}
		}
		return
	}
	for x := rect.Min.X; x < rect.Min.X+width && x < rect.Max.X; x++ {
		for y := rect.Min.Y; y < rect.Max.Y; y++ {
			canvas.SetGray(x, y, gray)
		}
	}
}

func parseColor(value string, fallback color.Color) color.Color {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return fallback
	}
	if value == "transparent" {
		return color.Transparent
	}
	if strings.HasPrefix(value, "#") {
		hex := strings.TrimPrefix(value, "#")
		if len(hex) == 3 {
			hex = strings.Map(func(r rune) rune { return r }, strings.Repeat(string(hex[0]), 2)+strings.Repeat(string(hex[1]), 2)+strings.Repeat(string(hex[2]), 2))
		}
		if len(hex) == 6 {
			if parsed, err := strconv.ParseUint(hex, 16, 32); err == nil {
				return color.RGBA{uint8(parsed >> 16), uint8(parsed >> 8), uint8(parsed), 255}
			}
		}
	}
	switch value {
	case "white":
		return color.White
	case "black":
		return color.Black
	case "gray", "grey":
		return color.Gray{Y: 128}
	default:
		return fallback
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func sameOrigin(baseURL, imageURL string) bool {
	base, baseErr := url.Parse(baseURL)
	image, imageErr := url.Parse(imageURL)
	if baseErr != nil || imageErr != nil || base.Host == "" || image.Host == "" {
		return false
	}
	return strings.EqualFold(base.Scheme, image.Scheme) && strings.EqualFold(base.Host, image.Host)
}

func init() {
	// Register the formats supported by Kindle dashboard images.
	image.RegisterFormat("png", "\x89PNG", png.Decode, png.DecodeConfig)
	image.RegisterFormat("jpeg", "\xff\xd8", jpeg.Decode, jpeg.DecodeConfig)
	image.RegisterFormat("gif", "GIF8", gif.Decode, gif.DecodeConfig)
}
