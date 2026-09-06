package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kgym-hina/kindle-ha-dashboard/kindle/internal/ha"
	"github.com/kgym-hina/kindle-ha-dashboard/kindle/internal/model"
)

func (a *App) refreshBoundStates(ctx context.Context) error {
	ids := a.boundEntityIDs()
	var refreshErrors []string
	for _, entityID := range ids {
		state, err := a.ha.FetchState(ctx, entityID)
		if err != nil {
			refreshErrors = append(refreshErrors, fmt.Sprintf("%s: %v", entityID, err))
			continue
		}
		a.handleState(ctx, *state)
	}
	if len(refreshErrors) > 0 {
		return errors.New(strings.Join(refreshErrors, "; "))
	}
	return nil
}

func (a *App) boundEntityIDs() []string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	seen := make(map[string]struct{})
	for _, document := range []*model.Document{a.portable, a.zoneDocument, a.current} {
		for _, entityID := range model.BoundEntityIDs(document) {
			seen[entityID] = struct{}{}
		}
	}
	ids := make([]string, 0, len(seen))
	for entityID := range seen {
		ids = append(ids, entityID)
	}
	sort.Strings(ids)
	return ids
}

func (a *App) updateBoundState(state ha.State) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.isBoundEntityLocked(state.EntityID) {
		return false
	}
	next := model.EntityState{State: state.State, Attributes: state.Attributes}
	previous, exists := a.boundStates[state.EntityID]
	a.boundStates[state.EntityID] = next
	return !exists || !reflect.DeepEqual(previous, next)
}

func (a *App) isBoundEntityLocked(entityID string) bool {
	for _, document := range []*model.Document{a.portable, a.zoneDocument, a.current} {
		for _, boundID := range model.BoundEntityIDs(document) {
			if boundID == entityID {
				return true
			}
		}
	}
	return false
}

func cloneStateMap(states map[string]model.EntityState) map[string]model.EntityState {
	if len(states) == 0 {
		return nil
	}
	copy := make(map[string]model.EntityState, len(states))
	for entityID, state := range states {
		attributes := make(map[string]any, len(state.Attributes))
		for key, value := range state.Attributes {
			attributes[key] = value
		}
		copy[entityID] = model.EntityState{State: state.State, Attributes: attributes}
	}
	return copy
}

func visibleStateMap(document *model.Document, pageIndex int, states map[string]model.EntityState) map[string]model.EntityState {
	if document == nil || len(states) == 0 {
		return nil
	}
	page := document.Page(pageIndex)
	visible := make(map[string]model.EntityState)
	for _, element := range page.Elements {
		if element.Binding == nil {
			continue
		}
		if state, ok := states[element.Binding.EntityID]; ok {
			visible[element.Binding.EntityID] = state
		}
	}
	return cloneStateMap(visible)
}

func stateMapsEqual(left, right map[string]model.EntityState) bool {
	return reflect.DeepEqual(left, right)
}

func (a *App) toggleSwitch(ctx context.Context, element model.Element) {
	if element.Binding == nil {
		return
	}
	entityID := strings.TrimSpace(element.Binding.EntityID)
	if entityID == "" {
		return
	}
	if err := a.executeService(ctx, "switch", "toggle", map[string]any{"entity_id": entityID}); err != nil {
		logServiceError("switch.toggle", err)
		return
	}
	select {
	case <-time.After(180 * time.Millisecond):
	case <-ctx.Done():
		return
	}
	refreshCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if state, err := a.ha.FetchState(refreshCtx, entityID); err == nil {
		a.handleState(ctx, *state)
	} else if !errors.Is(err, context.Canceled) {
		logServiceError("switch.refresh", err)
	}
}

func (a *App) callService(ctx context.Context, domain, service string, serviceData map[string]any) {
	if err := a.executeService(ctx, domain, service, serviceData); err != nil {
		log.Printf("call service %s.%s: %v", domain, service, err)
	}
}

func (a *App) executeService(ctx context.Context, domain, service string, serviceData map[string]any) error {
	callCtx, cancel := context.WithTimeout(ctx, serviceTimeout)
	defer cancel()
	return a.ha.CallService(callCtx, domain, service, serviceData)
}

func (a *App) handleClimateTouch(ctx context.Context, element model.Element, x, y int) {
	if element.Binding == nil || strings.TrimSpace(element.Binding.EntityID) == "" {
		return
	}
	entityID := element.Binding.EntityID
	frame := element.Frame
	current, modes := a.climateState(entityID)
	layout := model.ClimateHitLayout(frame, modes)
	if !contains(frame, x, y) {
		return
	}
	if contains(layout.Power, x, y) {
		go a.toggleClimatePower(ctx, entityID, current, modes)
		return
	}
	step := 0.5
	if element.Climate != nil && element.Climate.TemperatureStep > 0 {
		step = element.Climate.TemperatureStep
	}
	if contains(layout.Decrease, x, y) {
		go a.adjustClimateTemperature(ctx, entityID, -step)
		return
	}
	if contains(layout.Increase, x, y) {
		go a.adjustClimateTemperature(ctx, entityID, step)
		return
	}
	for _, mode := range layout.ModeItems {
		if contains(mode.Frame, x, y) {
			go a.setClimateMode(ctx, entityID, mode.Mode)
			return
		}
	}
}

func (a *App) climateState(entityID string) (string, []string) {
	a.mu.RLock()
	state, ok := a.boundStates[entityID]
	a.mu.RUnlock()
	if !ok {
		return "", model.ClimateModes(nil)
	}
	return model.ClimateCurrentMode(state), model.ClimateModes(state.Attributes)
}

func (a *App) adjustClimateTemperature(ctx context.Context, entityID string, delta float64) {
	target := 20.0
	a.mu.RLock()
	if state, ok := a.boundStates[entityID]; ok {
		target = numericAttribute(state.Attributes, "temperature", target)
	}
	a.mu.RUnlock()
	target = roundToStep(target+delta, 0.1)
	callCtx, cancel := context.WithTimeout(ctx, serviceTimeout)
	defer cancel()
	if err := a.ha.CallService(callCtx, "climate", "set_temperature", map[string]any{"entity_id": entityID, "temperature": target}); err != nil {
		logServiceError("climate.set_temperature", err)
		return
	}
	a.refreshClimateState(ctx, entityID)
}

func (a *App) toggleClimatePower(ctx context.Context, entityID, current string, modes []string) {
	if current != "" && !strings.EqualFold(current, "off") {
		a.setClimateMode(ctx, entityID, "off")
		return
	}
	mode := a.lastClimateMode(entityID, modes)
	if mode == "" {
		return
	}
	a.setClimateMode(ctx, entityID, mode)
}

func (a *App) lastClimateMode(entityID string, modes []string) string {
	a.mu.RLock()
	last := a.climateLastModes[entityID]
	a.mu.RUnlock()
	if climateModeSupported(last, modes) {
		return last
	}
	for _, preferred := range []string{"auto", "heat", "cool", "dry", "fan_only", "heat_cool"} {
		if climateModeSupported(preferred, modes) {
			return preferred
		}
	}
	if len(modes) > 0 {
		return modes[0]
	}
	return ""
}

func climateModeSupported(mode string, modes []string) bool {
	for _, candidate := range modes {
		if strings.EqualFold(mode, candidate) {
			return true
		}
	}
	return false
}

func (a *App) setClimateMode(ctx context.Context, entityID, mode string) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		return
	}
	callCtx, cancel := context.WithTimeout(ctx, serviceTimeout)
	defer cancel()
	if err := a.ha.CallService(callCtx, "climate", "set_hvac_mode", map[string]any{"entity_id": entityID, "hvac_mode": mode}); err != nil {
		logServiceError("climate.set_hvac_mode", err)
		return
	}
	if mode != "off" {
		a.mu.Lock()
		a.climateLastModes[entityID] = mode
		a.mu.Unlock()
	}
	a.refreshClimateState(ctx, entityID)
}

func (a *App) refreshClimateState(ctx context.Context, entityID string) {
	refreshCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if state, err := a.ha.FetchState(refreshCtx, entityID); err == nil {
		a.handleState(ctx, *state)
	} else if !errors.Is(err, context.Canceled) {
		logServiceError("climate.refresh", err)
	}
}

func stateAttributeString(attributes map[string]any, key, fallback string) string {
	if value, ok := attributes[key]; ok {
		return fmt.Sprint(value)
	}
	return fallback
}

const serviceTimeout = 8 * time.Second

func logServiceError(name string, err error) {
	log.Printf("%s: %v", name, err)
}

func numericAttribute(attributes map[string]any, key string, fallback float64) float64 {
	value, ok := attributes[key]
	if !ok {
		return fallback
	}
	parsed, err := strconv.ParseFloat(fmt.Sprint(value), 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func roundToStep(value, step float64) float64 {
	if step <= 0 {
		return value
	}
	return float64(int(value/step+0.5)) * step
}
