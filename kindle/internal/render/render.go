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

func (r *Renderer) Render(document *model.Document, pageIndex int, dialog *Dialog) (*image.Gray, error) {
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
		if err := r.drawElement(canvas, element); err != nil {
			return nil, fmt.Errorf("render element %s: %w", element.ID, err)
		}
	}
	if dialog != nil {
		r.drawDialog(canvas, *dialog)
	}
	return canvas, nil
}

func (r *Renderer) drawElement(canvas *image.Gray, element model.Element) error {
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
		r.drawTextBox(canvas, element.Text, frame, style)
	case "button":
		if fillColor != color.Transparent {
			fillRounded(canvas, frame, fillColor, maxInt(int(style.Radius), 0))
		}
		strokeRounded(canvas, frame, strokeColor, maxInt(int(style.Radius), 0), lineWidth)
		r.drawTextBox(canvas, element.Text, frame, style)
	case "image", "image_button":
		if element.Image == nil || strings.TrimSpace(element.Image.Src) == "" {
			return errors.New("image element is missing image.src")
		}
		asset, err := r.loadImage(element.Image.Src)
		if err != nil {
			return err
		}
		drawImage(canvas, frame, asset, element.Image.Fit)
		if element.Type == "image_button" {
			strokeRounded(canvas, frame, strokeColor, maxInt(int(style.Radius), 0), lineWidth)
			if element.Text != "" {
				r.drawTextBox(canvas, element.Text, frame, style)
			}
		}
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
			data, err = base64.StdEncoding.DecodeString(payload)
		} else {
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
		return decodeImage(io.LimitReader(response.Body, 8<<20))
	}
	file, err := os.Open(filepath.Clean(source))
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return decodeImage(io.LimitReader(file, 8<<20))
}

func decodeImage(reader io.Reader) (image.Image, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
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
	gray := color.GrayModel.Convert(value).(color.Gray)
	for y := canvas.Bounds().Min.Y; y < canvas.Bounds().Max.Y; y++ {
		for x := canvas.Bounds().Min.X; x < canvas.Bounds().Max.X; x++ {
			canvas.SetGray(x, y, gray)
		}
	}
}

func fillRounded(canvas *image.Gray, rect image.Rectangle, value color.Color, radius int) {
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

func strokeRounded(canvas *image.Gray, rect image.Rectangle, value color.Color, radius, width int) {
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
