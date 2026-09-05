package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/kgym-hina/kindle-ha-dashboard/kindle/internal/config"
	"github.com/kgym-hina/kindle-ha-dashboard/kindle/internal/ha"
	"github.com/kgym-hina/kindle-ha-dashboard/kindle/internal/input"
	"github.com/kgym-hina/kindle-ha-dashboard/kindle/internal/model"
	"github.com/kgym-hina/kindle-ha-dashboard/kindle/internal/power"
	"github.com/kgym-hina/kindle-ha-dashboard/kindle/internal/render"
	"github.com/kgym-hina/kindle-ha-dashboard/kindle/internal/ui"
)

type App struct {
	cfg      config.Config
	ha       *ha.Client
	renderer render.Renderer
	display  ui.Display

	mu             sync.RWMutex
	portable       *model.Document
	zoneDocument   *model.Document
	current        *model.Document
	currentZone    string
	pageIndex      int
	dialog         *render.Dialog
	dialogDeadline time.Time
}

func New(cfg config.Config) *App {
	client := ha.NewClient(cfg.HAURL, cfg.LongLivedToken)
	return &App{
		cfg:      cfg,
		ha:       client,
		renderer: render.Renderer{HTTPClient: nil, HAURL: cfg.HAURL, Token: cfg.LongLivedToken, FontPath: cfg.FontPath},
		display:  ui.Display{TempDir: cfg.TempDir, Width: cfg.DisplayWidth, Height: cfg.DisplayHeight},
	}
}

func (a *App) Run(ctx context.Context) error {
	if err := a.loadInitial(ctx); err != nil {
		log.Printf("initial dashboard load: %v", err)
	}
	if err := a.renderNow(); err != nil {
		return fmt.Errorf("initial render: %w", err)
	}

	stateCh := make(chan ha.State, 16)
	messageCh := make(chan ha.Message, 8)
	clientErrCh := make(chan error, 1)
	go func() {
		clientErrCh <- a.ha.Run(ctx, func(state ha.State) {
			select {
			case stateCh <- state:
			case <-ctx.Done():
			}
		}, func(message ha.Message) {
			select {
			case messageCh <- message:
			case <-ctx.Done():
			}
		})
	}()

	touchCh := make(chan input.TouchEvent, 8)
	touchErrCh := make(chan error, 1)
	go func() {
		touchErrCh <- (input.Reader{DevicePath: a.cfg.InputDevice, MaxX: a.cfg.TouchWidth, MaxY: a.cfg.TouchHeight}).Read(ctx, touchCh)
	}()

	batteryTicker := time.NewTicker(time.Duration(a.cfg.BatteryIntervalSec) * time.Second)
	defer batteryTicker.Stop()
	dialogTicker := time.NewTicker(500 * time.Millisecond)
	defer dialogTicker.Stop()
	resyncTicker := time.NewTicker(60 * time.Second)
	defer resyncTicker.Stop()
	if err := a.publishBattery(ctx); err != nil {
		log.Printf("initial battery report: %v", err)
	}

	var press *input.TouchEvent
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-clientErrCh:
			if err != nil && !errors.Is(err, context.Canceled) {
				log.Printf("home assistant websocket: %v", err)
			}
			clientErrCh = nil
		case err := <-touchErrCh:
			if err != nil && !errors.Is(err, context.Canceled) {
				return fmt.Errorf("touch reader: %w", err)
			}
			return nil
		case state := <-stateCh:
			a.handleState(ctx, state)
		case message := <-messageCh:
			a.handleMessage(message)
		case event := <-touchCh:
			if event.Pressed {
				copyEvent := event
				press = &copyEvent
				continue
			}
			if event.Released {
				if press != nil {
					if a.handleTouch(ctx, press.X, press.Y) {
						return nil
					}
				}
				press = nil
			}
		case <-batteryTicker.C:
			if err := a.publishBattery(ctx); err != nil {
				log.Printf("battery report: %v", err)
			}
		case <-resyncTicker.C:
			a.resync(ctx)
		case <-dialogTicker.C:
			if a.clearExpiredDialog() {
				if err := a.renderNow(); err != nil {
					log.Printf("render dialog timeout: %v", err)
				}
			}
		}
	}
}

func (a *App) resync(ctx context.Context) error {
	var refreshErrors []string
	if state, err := a.ha.FetchState(ctx, a.cfg.PortableEntity); err == nil {
		a.handleState(ctx, *state)
	} else {
		refreshErrors = append(refreshErrors, fmt.Sprintf("portable: %v", err))
	}
	if state, err := a.ha.FetchState(ctx, a.cfg.LocationEntity); err == nil && usableZone(state.State) {
		a.handleState(ctx, *state)
	} else if err != nil {
		refreshErrors = append(refreshErrors, fmt.Sprintf("location: %v", err))
	}
	a.mu.RLock()
	zone := a.currentZone
	a.mu.RUnlock()
	if usableZone(zone) {
		if state, err := a.ha.FetchState(ctx, a.cfg.ZoneEntity(zone)); err == nil {
			a.handleState(ctx, *state)
		} else {
			refreshErrors = append(refreshErrors, fmt.Sprintf("zone %s: %v", zone, err))
		}
	}
	if len(refreshErrors) > 0 {
		return errors.New(strings.Join(refreshErrors, "; "))
	}
	return nil
}

func (a *App) loadInitial(ctx context.Context) error {
	portableState, portableErr := a.ha.FetchState(ctx, a.cfg.PortableEntity)
	if portableErr == nil {
		if document, err := model.FromAttributes(portableState.Attributes); err == nil {
			a.mu.Lock()
			a.portable = document
			a.current = document
			a.mu.Unlock()
		} else {
			portableErr = err
		}
	}
	locationState, locationErr := a.ha.FetchState(ctx, a.cfg.LocationEntity)
	if locationErr == nil && usableZone(locationState.State) {
		a.switchZone(ctx, locationState.State)
	}
	if portableErr != nil && locationErr != nil {
		return fmt.Errorf("portable: %v; location: %v", portableErr, locationErr)
	}
	return portableErr
}

func (a *App) handleState(ctx context.Context, state ha.State) {
	switch state.EntityID {
	case a.cfg.PortableEntity:
		document, err := model.FromAttributes(state.Attributes)
		if err != nil {
			log.Printf("portable document update: %v", err)
			return
		}
		a.mu.Lock()
		a.portable = document
		shouldRender := a.zoneDocument == nil
		if shouldRender {
			a.current = document
			a.pageIndex = 0
		}
		a.mu.Unlock()
		if shouldRender {
			a.renderAndLog()
		}
	case a.cfg.LocationEntity:
		if usableZone(state.State) {
			a.switchZone(ctx, state.State)
			return
		}
		a.mu.Lock()
		a.currentZone = ""
		a.zoneDocument = nil
		a.current = a.portable
		a.pageIndex = 0
		a.mu.Unlock()
		a.renderAndLog()
	default:
		a.mu.RLock()
		currentZone := a.currentZone
		a.mu.RUnlock()
		if currentZone == "" || state.EntityID != a.cfg.ZoneEntity(currentZone) {
			return
		}
		document, err := model.FromAttributes(state.Attributes)
		if err != nil {
			log.Printf("zone document update: %v", err)
			return
		}
		a.mu.Lock()
		a.zoneDocument = document
		a.current = document
		a.pageIndex = 0
		a.mu.Unlock()
		a.renderAndLog()
	}
}

func (a *App) switchZone(ctx context.Context, zone string) {
	zone = strings.TrimSpace(zone)
	if !usableZone(zone) {
		return
	}
	fetchCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	state, err := a.ha.FetchState(fetchCtx, a.cfg.ZoneEntity(zone))
	var document *model.Document
	if err == nil {
		document, err = model.FromAttributes(state.Attributes)
	}
	a.mu.Lock()
	a.currentZone = zone
	a.zoneDocument = document
	if document != nil {
		a.current = document
	} else {
		a.current = a.portable
	}
	a.pageIndex = 0
	a.mu.Unlock()
	if err != nil {
		log.Printf("zone %s dashboard unavailable, using portable: %v", zone, err)
	}
	a.renderAndLog()
}

func (a *App) handleMessage(message ha.Message) {
	if message.TargetDeviceID != "" && message.TargetDeviceID != a.cfg.DeviceID {
		return
	}
	timeout := time.Duration(message.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	a.mu.Lock()
	a.dialog = &render.Dialog{Title: message.Title, Message: message.Message, ExpiresAt: time.Now().Add(timeout).UnixMilli()}
	a.dialogDeadline = time.Now().Add(timeout)
	a.mu.Unlock()
	a.renderAndLog()
}

func (a *App) handleTouch(ctx context.Context, x, y int) bool {
	if a.cfg.TouchWidth > 0 {
		x = x * a.cfg.DisplayWidth / a.cfg.TouchWidth
	}
	if a.cfg.TouchHeight > 0 {
		y = y * a.cfg.DisplayHeight / a.cfg.TouchHeight
	}
	a.mu.Lock()
	if a.dialog != nil {
		a.dialog = nil
		a.dialogDeadline = time.Time{}
		a.mu.Unlock()
		a.renderAndLog()
		return false
	}
	document := a.current
	pageIndex := a.pageIndex
	a.mu.Unlock()
	if document == nil {
		document = fallbackDocument(a.cfg.DisplayWidth, a.cfg.DisplayHeight)
	}
	if document.Target.Width > 0 && document.Target.Width != a.cfg.DisplayWidth {
		x = x * document.Target.Width / a.cfg.DisplayWidth
	}
	if document.Target.Height > 0 && document.Target.Height != a.cfg.DisplayHeight {
		y = y * document.Target.Height / a.cfg.DisplayHeight
	}
	page := document.Page(pageIndex)
	for index := len(page.Elements) - 1; index >= 0; index-- {
		element := page.Elements[index]
		if !contains(element.Frame, x, y) || element.Action == nil {
			continue
		}
		return a.runAction(ctx, element.Action)
	}
	return false
}

func (a *App) runAction(ctx context.Context, action *model.Action) bool {
	switch action.Type {
	case "navigate_page":
		a.mu.Lock()
		if a.current != nil {
			if pageIndex := a.current.PageIndex(action.PageID); pageIndex >= 0 {
				a.pageIndex = pageIndex
			}
		}
		a.mu.Unlock()
		a.renderAndLog()
		return false
	case "call_service":
		callCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
		defer cancel()
		if err := a.ha.CallService(callCtx, action.Domain, action.Service, action.ServiceData); err != nil {
			log.Printf("call service %s.%s: %v", action.Domain, action.Service, err)
		}
		return false
	case "show_message":
		a.handleMessage(ha.Message{Title: action.Title, Message: action.Message, TimeoutMS: action.TimeoutMS})
		return false
	case "refresh_config":
		refreshCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
		defer cancel()
		if err := a.resync(refreshCtx); err != nil {
			log.Printf("manual dashboard refresh: %v", err)
		}
		a.renderAndLog()
		return false
	case "exit":
		log.Printf("exit requested from waiting screen")
		return true
	default:
		log.Printf("unsupported action type %q", action.Type)
		return false
	}
}

func (a *App) publishBattery(ctx context.Context) error {
	percent, err := power.ReadPercent()
	if err != nil {
		return err
	}
	return a.ha.UpdateState(ctx, a.cfg.BatteryEntity, fmt.Sprintf("%d", percent), map[string]any{
		"friendly_name":       "Kindle Battery",
		"unit_of_measurement": "%",
		"device_class":        "battery",
		"state_class":         "measurement",
		"source":              "kindle-ha-dashboard",
	})
}

func (a *App) renderNow() error {
	a.mu.RLock()
	document := a.current
	pageIndex := a.pageIndex
	dialog := a.dialog
	a.mu.RUnlock()
	if document == nil {
		document = fallbackDocument(a.cfg.DisplayWidth, a.cfg.DisplayHeight)
	}
	canvas, err := a.renderer.Render(document, pageIndex, dialog)
	if err != nil {
		return err
	}
	return a.display.Show(canvas)
}

func (a *App) renderAndLog() {
	if err := a.renderNow(); err != nil {
		log.Printf("render dashboard: %v", err)
	}
}

func (a *App) clearExpiredDialog() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.dialog == nil || a.dialogDeadline.IsZero() || time.Now().Before(a.dialogDeadline) {
		return false
	}
	a.dialog = nil
	a.dialogDeadline = time.Time{}
	return true
}

func contains(frame model.Frame, x, y int) bool {
	return float64(x) >= frame.X && float64(x) <= frame.X+frame.Width && float64(y) >= frame.Y && float64(y) <= frame.Y+frame.Height
}

func usableZone(zone string) bool {
	zone = strings.ToLower(strings.TrimSpace(zone))
	return zone != "" && zone != "unknown" && zone != "unavailable" && zone != "none"
}

func fallbackDocument(width, height int) *model.Document {
	margin := 36
	gap := 18
	buttonHeight := 76
	buttonWidth := (width - margin*2 - gap) / 2
	if buttonWidth < 1 {
		buttonWidth = 1
	}
	buttonY := height/2 + 58
	return &model.Document{
		Schema:   model.Schema,
		Revision: 1,
		Target:   model.Target{Mode: "portable", Width: width, Height: height, Background: "#ffffff"},
		Pages: []model.Page{{ID: "waiting", Name: "Waiting", Elements: []model.Element{
			{
				ID: "waiting-text", Type: "text", Frame: model.Frame{X: 30, Y: float64(height/2 - 84), Width: float64(width - 60), Height: 44},
				Style: model.Style{Color: "#111111", FontSize: 22, Align: "center"}, Text: "Waiting for Home Assistant",
			},
			{
				ID: "waiting-hint", Type: "text", Frame: model.Frame{X: 30, Y: float64(height/2 - 24), Width: float64(width - 60), Height: 42},
				Style: model.Style{Color: "#555555", FontSize: 16, Align: "center"}, Text: "Check connection or refresh configuration",
			},
			{
				ID: "waiting-refresh", Type: "button", Frame: model.Frame{X: float64(margin), Y: float64(buttonY), Width: float64(buttonWidth), Height: float64(buttonHeight)},
				Style: model.Style{Color: "#ffffff", Fill: "#111111", Stroke: "#111111", BorderWidth: 2, Radius: 10, FontSize: 18, Align: "center"}, Text: "Refresh config",
				Action: &model.Action{Type: "refresh_config"},
			},
			{
				ID: "waiting-exit", Type: "button", Frame: model.Frame{X: float64(margin + buttonWidth + gap), Y: float64(buttonY), Width: float64(buttonWidth), Height: float64(buttonHeight)},
				Style: model.Style{Color: "#111111", Fill: "#ffffff", Stroke: "#111111", BorderWidth: 2, Radius: 10, FontSize: 18, Align: "center"}, Text: "Exit",
				Action: &model.Action{Type: "exit"},
			},
		}}},
	}
}
