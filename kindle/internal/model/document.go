package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

const Schema = "kindle-dashboard/v1"

type Document struct {
	Schema    string `json:"schema"`
	Revision  int64  `json:"revision"`
	UpdatedAt string `json:"updated_at,omitempty"`
	Target    Target `json:"target"`
	Pages     []Page `json:"pages"`
}

type Target struct {
	Mode       string  `json:"mode"`
	ZoneID     *string `json:"zone_id,omitempty"`
	Width      int     `json:"width"`
	Height     int     `json:"height"`
	Background string  `json:"background"`
}

type Page struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	ParentID   *string   `json:"parent_id,omitempty"`
	Background string    `json:"background,omitempty"`
	Elements   []Element `json:"elements"`
}

type Frame struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type Style struct {
	Color       string  `json:"color,omitempty"`
	Fill        string  `json:"fill,omitempty"`
	Stroke      string  `json:"stroke,omitempty"`
	BorderWidth float64 `json:"border_width,omitempty"`
	Radius      float64 `json:"radius,omitempty"`
	FontSize    float64 `json:"font_size,omitempty"`
	FontWeight  string  `json:"font_weight,omitempty"`
	Align       string  `json:"align,omitempty"`
}

type Image struct {
	Src string `json:"src"`
	Fit string `json:"fit,omitempty"`
}

type Action struct {
	Type        string         `json:"type"`
	PageID      string         `json:"page_id,omitempty"`
	Domain      string         `json:"domain,omitempty"`
	Service     string         `json:"service,omitempty"`
	ServiceData map[string]any `json:"service_data,omitempty"`
	Title       string         `json:"title,omitempty"`
	Message     string         `json:"message,omitempty"`
	TimeoutMS   int            `json:"timeout_ms,omitempty"`
}

type Binding struct {
	EntityID string `json:"entity_id"`
	Field    string `json:"field,omitempty"`
	Prefix   string `json:"prefix,omitempty"`
	Suffix   string `json:"suffix,omitempty"`
	Decimals *int   `json:"decimals,omitempty"`
}

type ClimateOptions struct {
	TemperatureStep float64 `json:"temperature_step,omitempty"`
}

type Element struct {
	ID      string          `json:"id"`
	Type    string          `json:"type"`
	Frame   Frame           `json:"frame"`
	Style   Style           `json:"style,omitempty"`
	Text    string          `json:"text,omitempty"`
	Image   *Image          `json:"image,omitempty"`
	Binding *Binding        `json:"binding,omitempty"`
	Climate *ClimateOptions `json:"climate,omitempty"`
	Action  *Action         `json:"action,omitempty"`
}

type EntityState struct {
	State      string         `json:"state"`
	Attributes map[string]any `json:"attributes"`
}

type Message struct {
	TargetDeviceID string `json:"target_device_id,omitempty"`
	Title          string `json:"title"`
	Message        string `json:"message"`
	TimeoutMS      int    `json:"timeout_ms,omitempty"`
}

func Decode(data []byte) (*Document, error) {
	var document Document
	if err := json.Unmarshal(data, &document); err != nil {
		return nil, err
	}
	if err := document.Validate(); err != nil {
		return nil, err
	}
	return &document, nil
}

func FromAttributes(attributes map[string]any) (*Document, error) {
	raw, ok := attributes["document"]
	if !ok {
		return nil, errors.New("state does not contain a document attribute")
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("encode document attribute: %w", err)
	}
	return Decode(encoded)
}

func Equal(left, right *Document) bool {
	if left == nil || right == nil {
		return left == right
	}
	return reflect.DeepEqual(*left, *right)
}

func BoundEntityIDs(document *Document) []string {
	if document == nil {
		return nil
	}
	seen := make(map[string]struct{})
	for _, page := range document.Pages {
		for _, element := range page.Elements {
			if element.Binding != nil && strings.TrimSpace(element.Binding.EntityID) != "" {
				seen[element.Binding.EntityID] = struct{}{}
			}
		}
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func (d *Document) Validate() error {
	if d == nil {
		return errors.New("document is nil")
	}
	if d.Schema != Schema {
		return fmt.Errorf("unsupported schema %q", d.Schema)
	}
	if d.Revision < 1 {
		return errors.New("revision must be at least 1")
	}
	if d.Target.Mode != "portable" && d.Target.Mode != "zone" {
		return fmt.Errorf("target.mode must be portable or zone, got %q", d.Target.Mode)
	}
	if d.Target.Mode == "zone" && strings.TrimSpace(pointerValue(d.Target.ZoneID)) == "" {
		return errors.New("zone document requires target.zone_id")
	}
	if d.Target.Width <= 0 || d.Target.Height <= 0 {
		return errors.New("target dimensions must be positive")
	}
	if len(d.Pages) == 0 {
		return errors.New("document must contain at least one page")
	}
	pageIDs := make(map[string]struct{}, len(d.Pages))
	for pageIndex, page := range d.Pages {
		if strings.TrimSpace(page.ID) == "" {
			return fmt.Errorf("page %d has an empty id", pageIndex)
		}
		if _, exists := pageIDs[page.ID]; exists {
			return fmt.Errorf("page %q is duplicated", page.ID)
		}
		pageIDs[page.ID] = struct{}{}
	}
	for pageIndex, page := range d.Pages {
		if page.ParentID != nil {
			parentID := strings.TrimSpace(*page.ParentID)
			if parentID == "" {
				return fmt.Errorf("page %d has an empty parent_id", pageIndex)
			}
			if parentID == page.ID {
				return fmt.Errorf("page %q cannot be its own parent", page.ID)
			}
			if _, exists := pageIDs[parentID]; !exists {
				return fmt.Errorf("page %q references missing parent %q", page.ID, parentID)
			}
		}
	}
	for _, page := range d.Pages {
		visited := make(map[string]bool)
		currentID := page.ID
		for {
			if visited[currentID] {
				return fmt.Errorf("page hierarchy contains a cycle at %q", currentID)
			}
			visited[currentID] = true
			currentIndex := d.PageIndex(currentID)
			if currentIndex < 0 || d.Pages[currentIndex].ParentID == nil || strings.TrimSpace(*d.Pages[currentIndex].ParentID) == "" {
				break
			}
			currentID = strings.TrimSpace(*d.Pages[currentIndex].ParentID)
		}
	}
	for _, page := range d.Pages {
		for elementIndex, element := range page.Elements {
			if strings.TrimSpace(element.ID) == "" {
				return fmt.Errorf("page %q element %d has an empty id", page.ID, elementIndex)
			}
			if element.Frame.Width <= 0 || element.Frame.Height <= 0 {
				return fmt.Errorf("page %q element %q has an invalid frame", page.ID, element.ID)
			}
			if !validElementType(element.Type) {
				return fmt.Errorf("page %q element %q has unsupported type %q", page.ID, element.ID, element.Type)
			}
			if (element.Type == "image" || element.Type == "image_button") && (element.Image == nil || strings.TrimSpace(element.Image.Src) == "") {
				return fmt.Errorf("page %q image element %q is missing image.src", page.ID, element.ID)
			}
			if element.Binding != nil {
				if strings.TrimSpace(element.Binding.EntityID) == "" {
					return fmt.Errorf("page %q element %q binding requires entity_id", page.ID, element.ID)
				}
				if element.Binding.Decimals != nil && (*element.Binding.Decimals < 0 || *element.Binding.Decimals > 6) {
					return fmt.Errorf("page %q element %q binding.decimals must be between 0 and 6", page.ID, element.ID)
				}
			}
			if element.Climate != nil && element.Climate.TemperatureStep <= 0 {
				return fmt.Errorf("page %q element %q climate.temperature_step must be positive", page.ID, element.ID)
			}
			if err := validateAction(element.Action); err != nil {
				return fmt.Errorf("page %q element %q action: %w", page.ID, element.ID, err)
			}
		}
	}
	return nil
}

func (d *Document) Page(index int) Page {
	if len(d.Pages) == 0 {
		return Page{}
	}
	if index < 0 || index >= len(d.Pages) {
		index = 0
	}
	return d.Pages[index]
}

func (d *Document) PageIndex(pageID string) int {
	for index, page := range d.Pages {
		if page.ID == pageID {
			return index
		}
	}
	return -1
}

func validElementType(value string) bool {
	switch value {
	case "text", "button", "image_button", "image", "rect", "line", "switch", "climate":
		return true
	default:
		return false
	}
}

func validateAction(action *Action) error {
	if action == nil {
		return nil
	}
	switch action.Type {
	case "navigate_page":
		if strings.TrimSpace(action.PageID) == "" {
			return errors.New("navigate_page requires page_id")
		}
	case "call_service":
		if strings.TrimSpace(action.Domain) == "" || strings.TrimSpace(action.Service) == "" {
			return errors.New("call_service requires domain and service")
		}
	case "show_message":
		if strings.TrimSpace(action.Message) == "" {
			return errors.New("show_message requires message")
		}
	case "refresh_config", "exit":
		// These actions are reserved for the Kindle fallback page.
	default:
		return fmt.Errorf("unsupported type %q", action.Type)
	}
	return nil
}

func pointerValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
