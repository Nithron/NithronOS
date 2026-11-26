package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	"nos/backend/nosd/internal/config"
	"nos/backend/nosd/pkg/httpx"
)

// AppearanceSettings represents UI appearance configuration
type AppearanceSettings struct {
	Theme          string            `json:"theme"`        // light, dark, auto
	AccentColor    string            `json:"accent_color"` // hex color
	FontSize       string            `json:"font_size"`    // small, medium, large
	ReducedMotion  bool              `json:"reduced_motion"`
	HighContrast   bool              `json:"high_contrast"`
	Language       string            `json:"language"`          // en, es, fr, de, etc.
	DateFormat     string            `json:"date_format"`       // MM/DD/YYYY, DD/MM/YYYY, YYYY-MM-DD
	TimeFormat     string            `json:"time_format"`       // 12h, 24h
	FirstDayOfWeek int               `json:"first_day_of_week"` // 0=Sunday, 1=Monday
	CustomCSS      string            `json:"custom_css"`
	CustomColors   map[string]string `json:"custom_colors"`
}

// AppearanceHandler handles appearance settings
type AppearanceHandler struct {
	config       config.Config
	settingsPath string
}

// NewAppearanceHandler creates a new appearance handler
func NewAppearanceHandler(cfg config.Config) *AppearanceHandler {
	return &AppearanceHandler{
		config:       cfg,
		settingsPath: filepath.Join(cfg.EtcDir, "nos", "appearance.json"),
	}
}

// Routes returns the routes for the appearance handler
func (h *AppearanceHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.GetAppearanceSettings)
	r.Put("/", h.UpdateAppearanceSettings)
	return r
}

// GetAppearanceSettings returns current appearance settings
func (h *AppearanceHandler) GetAppearanceSettings(w http.ResponseWriter, r *http.Request) {
	settings := h.loadSettings()
	writeJSON(w, settings)
}

// UpdateAppearanceSettings updates appearance settings
func (h *AppearanceHandler) UpdateAppearanceSettings(w http.ResponseWriter, r *http.Request) {
	var settings AppearanceSettings
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		httpx.WriteTypedError(w, http.StatusBadRequest, "appearance.invalid_request", "Invalid request body", 0)
		return
	}

	// Validate theme
	if settings.Theme != "light" && settings.Theme != "dark" && settings.Theme != "auto" {
		httpx.WriteTypedError(w, http.StatusBadRequest, "appearance.invalid_theme", "Invalid theme", 0)
		return
	}

	// Validate language
	validLanguages := []string{"en", "es", "fr", "de", "it", "pt", "ru", "zh", "ja", "ko"}
	if !contains(validLanguages, settings.Language) {
		settings.Language = "en"
	}

	// Validate date format
	validDateFormats := []string{"MM/DD/YYYY", "DD/MM/YYYY", "YYYY-MM-DD", "DD.MM.YYYY"}
	if !contains(validDateFormats, settings.DateFormat) {
		settings.DateFormat = "MM/DD/YYYY"
	}

	// Validate time format
	if settings.TimeFormat != "12h" && settings.TimeFormat != "24h" {
		settings.TimeFormat = "12h"
	}

	// Save settings
	if err := h.saveSettings(settings); err != nil {
		httpx.WriteTypedError(w, http.StatusInternalServerError, "appearance.save_failed", "Failed to save settings", 0)
		return
	}

	writeJSON(w, settings)
}

// GetThemePresets returns available theme presets
func (h *AppearanceHandler) GetThemePresets(w http.ResponseWriter, r *http.Request) {
	presets := []map[string]any{
		{
			"id":          "default",
			"name":        "Default",
			"description": "nOS default theme",
			"colors": map[string]string{
				"primary":    "#0066cc",
				"secondary":  "#6c757d",
				"success":    "#28a745",
				"danger":     "#dc3545",
				"warning":    "#ffc107",
				"info":       "#17a2b8",
				"background": "#ffffff",
				"foreground": "#212529",
			},
		},
		{
			"id":          "dark",
			"name":        "Dark",
			"description": "Dark theme for low-light environments",
			"colors": map[string]string{
				"primary":    "#0d6efd",
				"secondary":  "#6c757d",
				"success":    "#198754",
				"danger":     "#dc3545",
				"warning":    "#ffc107",
				"info":       "#0dcaf0",
				"background": "#212529",
				"foreground": "#ffffff",
			},
		},
		{
			"id":          "high-contrast",
			"name":        "High Contrast",
			"description": "High contrast theme for accessibility",
			"colors": map[string]string{
				"primary":    "#000000",
				"secondary":  "#666666",
				"success":    "#008000",
				"danger":     "#ff0000",
				"warning":    "#ffff00",
				"info":       "#0000ff",
				"background": "#ffffff",
				"foreground": "#000000",
			},
		},
		{
			"id":          "ocean",
			"name":        "Ocean",
			"description": "Cool blue ocean theme",
			"colors": map[string]string{
				"primary":    "#006994",
				"secondary":  "#5e8ca6",
				"success":    "#00a86b",
				"danger":     "#ff6b6b",
				"warning":    "#ffd93d",
				"info":       "#4ecdc4",
				"background": "#f7f9fc",
				"foreground": "#2c3e50",
			},
		},
		{
			"id":          "forest",
			"name":        "Forest",
			"description": "Natural green forest theme",
			"colors": map[string]string{
				"primary":    "#2d5016",
				"secondary":  "#6b8e23",
				"success":    "#228b22",
				"danger":     "#8b0000",
				"warning":    "#daa520",
				"info":       "#4682b4",
				"background": "#f5f5dc",
				"foreground": "#2d3e0f",
			},
		},
	}

	writeJSON(w, presets)
}

// GetLanguages returns available languages
func (h *AppearanceHandler) GetLanguages(w http.ResponseWriter, r *http.Request) {
	languages := []map[string]string{
		{"code": "en", "name": "English", "native": "English"},
		{"code": "es", "name": "Spanish", "native": "Español"},
		{"code": "fr", "name": "French", "native": "Français"},
		{"code": "de", "name": "German", "native": "Deutsch"},
		{"code": "it", "name": "Italian", "native": "Italiano"},
		{"code": "pt", "name": "Portuguese", "native": "Português"},
		{"code": "ru", "name": "Russian", "native": "Русский"},
		{"code": "zh", "name": "Chinese", "native": "中文"},
		{"code": "ja", "name": "Japanese", "native": "日本語"},
		{"code": "ko", "name": "Korean", "native": "한국어"},
	}

	writeJSON(w, languages)
}

// Helper methods

func (h *AppearanceHandler) loadSettings() AppearanceSettings {
	settings := AppearanceSettings{
		Theme:          "auto",
		AccentColor:    "#0066cc",
		FontSize:       "medium",
		ReducedMotion:  false,
		HighContrast:   false,
		Language:       "en",
		DateFormat:     "MM/DD/YYYY",
		TimeFormat:     "12h",
		FirstDayOfWeek: 0,
		CustomColors:   make(map[string]string),
	}

	if data, err := os.ReadFile(h.settingsPath); err == nil {
		_ = json.Unmarshal(data, &settings)
	}

	return settings
}

func (h *AppearanceHandler) saveSettings(settings AppearanceSettings) error {
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(h.settingsPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(h.settingsPath, data, 0644)
}
