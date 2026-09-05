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
	if element.Binding == nil || strings.TrimSpace(element.Binding.EntityID) == "" {
		return
	}
	callCtx, cancel := context.WithTimeout(ctx, serviceTimeout)
	defer cancel()
	if err := a.ha.CallService(callCtx, "switch", "toggle", map[string]any{"entity_id": element.Binding.EntityID}); err != nil {
		logServiceError("switch.toggle", err)
	}
}

func (a *App) handleClimateTouch(ctx context.Context, element model.Element, x, y int) {
	if element.Binding == nil || strings.TrimSpace(element.Binding.EntityID) == "" {
		return
	}
	frame := element.Frame
	relativeX := float64(x) - frame.X
	relativeY := float64(y) - frame.Y
	if relativeX < 0 || relativeY < 0 || relativeX > frame.Width || relativeY > frame.Height {
		return
	}
	if relativeY >= frame.Height*0.62 {
		step := 0.5
		if element.Climate != nil && element.Climate.TemperatureStep > 0 {
			step = element.Climate.TemperatureStep
		}
		if relativeX < frame.Width/3 {
			a.adjustClimateTemperature(ctx, element.Binding.EntityID, -step)
			return
		}
		if relativeX > frame.Width*2/3 {
			a.adjustClimateTemperature(ctx, element.Binding.EntityID, step)
			return
		}
	}
	if relativeY < frame.Height*0.42 && relativeX > frame.Width*0.58 {
		a.cycleClimateMode(ctx, element.Binding.EntityID)
	}
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
	}
}

func (a *App) cycleClimateMode(ctx context.Context, entityID string) {
	current := ""
	modes := []string{"off", "heat", "cool", "auto"}
	a.mu.RLock()
	if state, ok := a.boundStates[entityID]; ok {
		current = strings.ToLower(stateAttributeString(state.Attributes, "hvac_mode", state.State))
		if values, ok := state.Attributes["hvac_modes"].([]any); ok && len(values) > 0 {
			modes = modes[:0]
			for _, value := range values {
				if text := strings.TrimSpace(fmt.Sprint(value)); text != "" {
					modes = append(modes, text)
				}
			}
		}
	}
	a.mu.RUnlock()
	if len(modes) == 0 {
		return
	}
	next := modes[0]
	for index, mode := range modes {
		if strings.EqualFold(mode, current) {
			next = modes[(index+1)%len(modes)]
			break
		}
	}
	callCtx, cancel := context.WithTimeout(ctx, serviceTimeout)
	defer cancel()
	if err := a.ha.CallService(callCtx, "climate", "set_hvac_mode", map[string]any{"entity_id": entityID, "hvac_mode": next}); err != nil {
		logServiceError("climate.set_hvac_mode", err)
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
