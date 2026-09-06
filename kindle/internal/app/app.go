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

	mu                     sync.RWMutex
	renderMu               sync.Mutex
	portable               *model.Document
	zoneDocument           *model.Document
	current                *model.Document
	currentZone            string
	pageIndex              int
	pageStack              []string
	settingsVisible        bool
	boundStates            map[string]model.EntityState
	dialog                 *render.Dialog
	dialogDeadline         time.Time
	pressedNavigation      int
	pressedNavigationToken uint64

	hasRendered                   bool
	lastRenderedDocument          *model.Document
	lastRenderedPage              int
	lastRenderedDialog            *render.Dialog
	lastRenderedSettings          bool
	lastRenderedCanGoBack         bool
	lastRenderedPressedNavigation int
	lastRenderedStates            map[string]model.EntityState
}

type inputResult struct {
	source string
	err    error
}

func New(cfg config.Config) *App {
	client := ha.NewClient(cfg.HAURL, cfg.LongLivedToken)
	return &App{
		cfg:                           cfg,
		ha:                            client,
		renderer:                      render.Renderer{HTTPClient: nil, HAURL: cfg.HAURL, Token: cfg.LongLivedToken, FontPath: cfg.FontPath, FontBoldPath: cfg.FontBoldPath},
		display:                       ui.Display{TempDir: cfg.TempDir, Width: cfg.DisplayWidth, Height: cfg.DisplayHeight},
		boundStates:                   make(map[string]model.EntityState),
		pressedNavigation:             -1,
		lastRenderedPressedNavigation: -1,
	}
}

func (a *App) Run(ctx context.Context) error {
	if err := a.display.Clear(); err != nil {
		log.Printf("clear display before loading: %v", err)
	}
	if err := a.loadInitial(ctx); err != nil {
		log.Printf("initial dashboard load: %v", err)
	}
	if err := a.renderFrame(true); err != nil {
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

	inputCh := make(chan input.Event, 16)
	inputErrCh := make(chan inputResult, 2)
	go func() {
		inputErrCh <- inputResult{source: "touch", err: (input.Reader{DevicePath: a.cfg.InputDevice, MaxX: a.cfg.TouchWidth, MaxY: a.cfg.TouchHeight, TouchOnly: true}).Read(ctx, inputCh)}
	}()
	if strings.TrimSpace(a.cfg.KeyDevice) != "" {
		go func() {
			inputErrCh <- inputResult{source: "physical key", err: (input.Reader{DevicePath: a.cfg.KeyDevice}).Read(ctx, inputCh)}
		}()
	}

	batteryTicker := time.NewTicker(time.Duration(a.cfg.BatteryIntervalSec) * time.Second)
	defer batteryTicker.Stop()
	dialogTicker := time.NewTicker(500 * time.Millisecond)
	defer dialogTicker.Stop()
	resyncTicker := time.NewTicker(time.Duration(a.cfg.DashboardRefreshIntervalSec) * time.Second)
	defer resyncTicker.Stop()
	forceRefreshTicker := time.NewTicker(time.Duration(a.cfg.ForceRefreshIntervalSec) * time.Second)
	defer forceRefreshTicker.Stop()
	if err := a.publishBattery(ctx); err != nil {
		log.Printf("initial battery report: %v", err)
	}

	var press *input.Event
	physicalKeyPressed := false
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-clientErrCh:
			if err != nil && !errors.Is(err, context.Canceled) {
				log.Printf("home assistant websocket: %v", err)
			}
			clientErrCh = nil
		case result := <-inputErrCh:
			if result.err != nil && !errors.Is(result.err, context.Canceled) {
				if result.source == "touch" {
					return fmt.Errorf("touch reader: %w", result.err)
				}
				log.Printf("%s reader: %v", result.source, result.err)
			}
		case state := <-stateCh:
			a.handleState(ctx, state)
		case message := <-messageCh:
			a.handleMessage(message)
		case event := <-inputCh:
			if event.KeyPressed || event.KeyReleased {
				if event.KeyPressed {
					physicalKeyPressed = true
				}
				if event.KeyReleased {
					if physicalKeyPressed {
						a.handlePhysicalKey()
					}
					physicalKeyPressed = false
				}
				continue
			}
			if event.Pressed {
				copyEvent := event
				press = &copyEvent
				a.handleTouchPress(event.X, event.Y)
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
		case <-forceRefreshTicker.C:
			a.forceRenderAndLog()
		case <-dialogTicker.C:
			if a.clearExpiredDialog() {
				if err := a.renderFrame(false); err != nil {
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
	if err := a.refreshBoundStates(ctx); err != nil {
		refreshErrors = append(refreshErrors, fmt.Sprintf("bound states: %v", err))
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
	if err := a.refreshBoundStates(ctx); err != nil {
		log.Printf("initial bound state load: %v", err)
	}
	if portableErr != nil && locationErr != nil {
		return fmt.Errorf("portable: %v; location: %v", portableErr, locationErr)
	}
	return portableErr
}

func (a *App) handleState(ctx context.Context, state ha.State) {
	boundChanged := a.updateBoundState(state)
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
		if shouldRender || boundChanged {
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
		a.pageStack = nil
		a.settingsVisible = false
		a.mu.Unlock()
		a.renderAndLog()
	default:
		a.mu.RLock()
		currentZone := a.currentZone
		a.mu.RUnlock()
		if currentZone == "" || state.EntityID != a.cfg.ZoneEntity(currentZone) {
			if boundChanged {
				a.renderAndLog()
			}
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
	a.pageStack = nil
	a.settingsVisible = false
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

func (a *App) handleTouchPress(x, y int) {
	x, y = a.normalizeTouch(x, y)
	a.mu.RLock()
	if a.dialog != nil {
		a.mu.RUnlock()
		return
	}
	document := a.current
	pageIndex := a.pageIndex
	settingsVisible := a.settingsVisible
	a.mu.RUnlock()
	if document == nil {
		document = fallbackDocument(a.cfg.DisplayWidth, a.cfg.DisplayHeight)
	}
	x, y = scaleTouchToDocument(x, y, document, a.cfg.DisplayWidth, a.cfg.DisplayHeight)
	page := document.Page(pageIndex)
	if settingsVisible {
		page = settingsDocument(document.Target.Width, document.Target.Height).Page(0)
	}
	if pageContains(page, x, y) {
		return
	}
	if y < document.Target.Height-render.NavigationBarHeight {
		return
	}
	index := navigationIndex(x, document.Target.Width)
	a.mu.Lock()
	a.pressedNavigation = index
	a.pressedNavigationToken++
	token := a.pressedNavigationToken
	a.mu.Unlock()
	time.AfterFunc(220*time.Millisecond, func() {
		a.mu.Lock()
		if a.pressedNavigationToken != token {
			a.mu.Unlock()
			return
		}
		a.pressedNavigation = -1
		a.mu.Unlock()
		a.renderAndLog()
	})
}

func (a *App) handleTouch(ctx context.Context, x, y int) bool {
	x, y = a.normalizeTouch(x, y)
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
	settingsVisible := a.settingsVisible
	a.mu.Unlock()
	if document == nil {
		document = fallbackDocument(a.cfg.DisplayWidth, a.cfg.DisplayHeight)
	}
	x, y = scaleTouchToDocument(x, y, document, a.cfg.DisplayWidth, a.cfg.DisplayHeight)
	page := document.Page(pageIndex)
	if settingsVisible {
		page = settingsDocument(document.Target.Width, document.Target.Height).Page(0)
	}
	for index := len(page.Elements) - 1; index >= 0; index-- {
		element := page.Elements[index]
		if !contains(element.Frame, x, y) {
			continue
		}
		if element.Type == "switch" {
			go a.toggleSwitch(ctx, element)
			return false
		}
		if element.Type == "climate" {
			go a.handleClimateTouch(ctx, element, x, y)
			return false
		}
		if element.Action == nil {
			continue
		}
		return a.runAction(ctx, element.Action)
	}
	if pageContains(page, x, y) {
		return false
	}
	if y >= document.Target.Height-render.NavigationBarHeight {
		return a.handleNavigationTouch(ctx, x, y, document.Target.Width)
	}
	return false
}

func (a *App) normalizeTouch(x, y int) (int, int) {
	if a.cfg.TouchWidth > 0 {
		x = x * a.cfg.DisplayWidth / a.cfg.TouchWidth
	}
	if a.cfg.TouchHeight > 0 {
		y = y * a.cfg.DisplayHeight / a.cfg.TouchHeight
	}
	return x, y
}

func scaleTouchToDocument(x, y int, document *model.Document, displayWidth, displayHeight int) (int, int) {
	if document == nil {
		return x, y
	}
	if document.Target.Width > 0 && document.Target.Width != displayWidth {
		x = x * document.Target.Width / displayWidth
	}
	if document.Target.Height > 0 && document.Target.Height != displayHeight {
		y = y * document.Target.Height / displayHeight
	}
	return x, y
}

func (a *App) runAction(ctx context.Context, action *model.Action) bool {
	switch action.Type {
	case "navigate_page":
		a.navigatePage(action.PageID)
		a.renderAndLog()
		return false
	case "call_service":
		go a.callService(ctx, action.Domain, action.Service, action.ServiceData)
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
		log.Printf("exit requested from settings")
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

func (a *App) renderFrame(force bool) error {
	a.renderMu.Lock()
	defer a.renderMu.Unlock()
	a.mu.RLock()
	document := a.current
	pageIndex := a.pageIndex
	dialog := cloneDialog(a.dialog)
	settingsVisible := a.settingsVisible
	canGoBack := a.canGoBackLocked()
	pressedNavigation := a.pressedNavigation
	states := cloneStateMap(a.boundStates)
	a.mu.RUnlock()
	if document == nil {
		document = fallbackDocument(a.cfg.DisplayWidth, a.cfg.DisplayHeight)
	}
	if settingsVisible {
		document = settingsDocument(a.cfg.DisplayWidth, a.cfg.DisplayHeight)
		pageIndex = 0
		states = nil
	}
	visibleStates := visibleStateMap(document, pageIndex, states)
	navigation := render.Navigation{Show: true, CanGoBack: canGoBack, IsSettings: settingsVisible, PressedIndex: pressedNavigation}
	if !force && a.renderedStateMatches(document, pageIndex, dialog, settingsVisible, canGoBack, pressedNavigation, visibleStates) {
		return nil
	}
	canvas, err := a.renderer.Render(document, pageIndex, dialog, navigation, visibleStates)
	if err != nil {
		return err
	}
	if err := a.display.Show(canvas, force); err != nil {
		return err
	}
	a.mu.Lock()
	a.hasRendered = true
	a.lastRenderedDocument = document
	a.lastRenderedPage = pageIndex
	a.lastRenderedDialog = dialog
	a.lastRenderedSettings = settingsVisible
	a.lastRenderedCanGoBack = canGoBack
	a.lastRenderedPressedNavigation = pressedNavigation
	a.lastRenderedStates = cloneStateMap(visibleStates)
	a.mu.Unlock()
	return nil
}

func (a *App) renderAndLog() {
	if err := a.renderFrame(false); err != nil {
		log.Printf("render dashboard: %v", err)
	}
}

func (a *App) forceRenderAndLog() {
	if err := a.renderFrame(true); err != nil {
		log.Printf("forced dashboard refresh: %v", err)
	}
}

func (a *App) renderedStateMatches(document *model.Document, pageIndex int, dialog *render.Dialog, settingsVisible, canGoBack bool, pressedNavigation int, states map[string]model.EntityState) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.hasRendered && a.lastRenderedPage == pageIndex && a.lastRenderedSettings == settingsVisible && a.lastRenderedCanGoBack == canGoBack && a.lastRenderedPressedNavigation == pressedNavigation && model.Equal(a.lastRenderedDocument, document) && dialogsEqual(a.lastRenderedDialog, dialog) && stateMapsEqual(a.lastRenderedStates, states)
}

func cloneDialog(dialog *render.Dialog) *render.Dialog {
	if dialog == nil {
		return nil
	}
	copy := *dialog
	return &copy
}

func dialogsEqual(left, right *render.Dialog) bool {
	if left == nil || right == nil {
		return left == right
	}
	return *left == *right
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

func pageContains(page model.Page, x, y int) bool {
	for index := len(page.Elements) - 1; index >= 0; index-- {
		if contains(page.Elements[index].Frame, x, y) {
			return true
		}
	}
	return false
}

func usableZone(zone string) bool {
	zone = strings.ToLower(strings.TrimSpace(zone))
	return zone != "" && zone != "unknown" && zone != "unavailable" && zone != "none"
}

func fallbackDocument(width, height int) *model.Document {
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
				Style: model.Style{Color: "#555555", FontSize: 16, Align: "center"}, Text: "底部设置菜单可刷新配置或退出",
			},
		}}},
	}
}
