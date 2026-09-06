package app

import (
	"context"
	"strings"

	"github.com/kgym-hina/kindle-ha-dashboard/kindle/internal/config"
	"github.com/kgym-hina/kindle-ha-dashboard/kindle/internal/model"
	"github.com/kgym-hina/kindle-ha-dashboard/kindle/internal/render"
)

func (a *App) handleNavigationTouch(_ context.Context, x, _ int, width int) bool {
	index := navigationIndex(x, width)
	if index < 0 {
		return false
	}
	switch index {
	case 0:
		a.goBack()
	case 1:
		a.goHome()
	case 2:
		a.openSettings()
	}
	return false
}

func navigationIndex(x, width int) int {
	if width <= 0 {
		return -1
	}
	index := x * 3 / width
	if index < 0 {
		return 0
	}
	if index > 2 {
		return 2
	}
	return index
}

func (a *App) navigatePage(pageID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.current == nil {
		return
	}
	pageIndex := a.current.PageIndex(pageID)
	if pageIndex < 0 || pageIndex == a.pageIndex {
		return
	}
	currentPage := a.current.Page(a.pageIndex)
	if strings.TrimSpace(currentPage.ID) != "" {
		a.pageStack = append(a.pageStack, currentPage.ID)
	}
	a.pageIndex = pageIndex
	a.settingsVisible = false
}

func (a *App) goBack() {
	a.mu.Lock()
	if a.settingsVisible {
		a.settingsVisible = false
		a.mu.Unlock()
		a.renderAndLog()
		return
	}
	if a.current == nil {
		a.mu.Unlock()
		return
	}
	if len(a.pageStack) > 0 {
		last := len(a.pageStack) - 1
		pageID := a.pageStack[last]
		a.pageStack = a.pageStack[:last]
		if pageIndex := a.current.PageIndex(pageID); pageIndex >= 0 {
			a.pageIndex = pageIndex
		}
	} else if parentID := a.current.Page(a.pageIndex).ParentID; parentID != nil && strings.TrimSpace(*parentID) != "" {
		if pageIndex := a.current.PageIndex(*parentID); pageIndex >= 0 {
			a.pageIndex = pageIndex
		}
	}
	a.mu.Unlock()
	a.renderAndLog()
}

func (a *App) goHome() {
	a.mu.Lock()
	if a.current != nil {
		pageIndex := a.current.PageIndex("home")
		if pageIndex < 0 {
			for index, page := range a.current.Pages {
				if page.ParentID == nil || strings.TrimSpace(pointerString(page.ParentID)) == "" {
					pageIndex = index
					break
				}
			}
		}
		if pageIndex >= 0 {
			a.pageIndex = pageIndex
		}
	}
	a.pageStack = nil
	a.settingsVisible = false
	a.mu.Unlock()
	a.renderAndLog()
}

func (a *App) openSettings() {
	a.mu.Lock()
	a.settingsVisible = true
	a.mu.Unlock()
	a.renderAndLog()
}

func (a *App) handlePhysicalKey() {
	a.mu.RLock()
	onHome := !a.settingsVisible && a.current != nil && isHomePage(a.current, a.pageIndex)
	a.mu.RUnlock()
	if onHome {
		a.openSettings()
		return
	}
	a.goHome()
}

func isHomePage(document *model.Document, pageIndex int) bool {
	if document == nil {
		return false
	}
	page := document.Page(pageIndex)
	if page.ID == "home" {
		return true
	}
	return page.ParentID == nil || strings.TrimSpace(pointerString(page.ParentID)) == ""
}

func (a *App) canGoBackLocked() bool {
	if a.settingsVisible || len(a.pageStack) > 0 {
		return true
	}
	if a.current == nil {
		return false
	}
	parentID := a.current.Page(a.pageIndex).ParentID
	return parentID != nil && strings.TrimSpace(*parentID) != ""
}

func settingsDocument(width, height int) *model.Document {
	contentHeight := height - render.NavigationBarHeight
	if contentHeight < 1 {
		contentHeight = height
	}
	buttonWidth := width - 72
	if buttonWidth < 1 {
		buttonWidth = 1
	}
	return &model.Document{
		Schema:   model.Schema,
		Revision: 1,
		Target:   model.Target{Mode: "portable", Width: width, Height: height, Background: "#ffffff"},
		Pages: []model.Page{{ID: "settings", Name: "Settings", Elements: []model.Element{
			{ID: "settings-title", Type: "text", Frame: model.Frame{X: 30, Y: 72, Width: float64(width - 60), Height: 54}, Style: model.Style{Color: "#111111", FontSize: 26, Align: "center"}, Text: "设置"},
			{ID: "settings-hint", Type: "text", Frame: model.Frame{X: 30, Y: 142, Width: float64(width - 60), Height: 44}, Style: model.Style{Color: "#555555", FontSize: 16, Align: "center"}, Text: "管理界面连接与程序"},
			{ID: "settings-version", Type: "text", Frame: model.Frame{X: 30, Y: 194, Width: float64(width - 60), Height: 30}, Style: model.Style{Color: "#777777", FontSize: 14, Align: "center"}, Text: "Kindle Dashboard v" + config.Version},
			{ID: "settings-refresh", Type: "button", Frame: model.Frame{X: 36, Y: float64(contentHeight/2 - 62), Width: float64(buttonWidth), Height: 72}, Style: model.Style{Color: "#ffffff", Fill: "#111111", Stroke: "#111111", BorderWidth: 2, Radius: 10, FontSize: 20, Align: "center"}, Text: "刷新配置", Action: &model.Action{Type: "refresh_config"}},
			{ID: "settings-exit", Type: "button", Frame: model.Frame{X: 36, Y: float64(contentHeight/2 + 30), Width: float64(buttonWidth), Height: 72}, Style: model.Style{Color: "#111111", Fill: "#ffffff", Stroke: "#111111", BorderWidth: 2, Radius: 10, FontSize: 20, Align: "center"}, Text: "退出 Dashboard", Action: &model.Action{Type: "exit"}},
		}}},
	}
}

func pointerString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
