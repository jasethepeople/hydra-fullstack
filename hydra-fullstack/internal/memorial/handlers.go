package memorial

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"

	"hydra-fullstack/internal/config"
	"hydra-fullstack/internal/metrics"
	"hydra-fullstack/internal/natsbus"
	"hydra-fullstack/internal/storage"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for memorial site
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// MemorialHandler handles memorial site API endpoints
type MemorialHandler struct {
	store     *storage.Store
	cfg       *config.MemorialConfig
	natsBus   *natsbus.NATSBus
	clients   map[*websocket.Conn]bool
	broadcast chan []byte
}

// NewMemorialHandler creates a new memorial handler
func NewMemorialHandler(store *storage.Store, cfg *config.MemorialConfig, natsBus *natsbus.NATSBus) *MemorialHandler {
	mh := &MemorialHandler{
		store:     store,
		cfg:       cfg,
		natsBus:   natsBus,
		clients:   make(map[*websocket.Conn]bool),
		broadcast: make(chan []byte),
	}
	go mh.handleBroadcasts()
	return mh
}

// RegisterRoutes sets up all memorial routes
func (mh *MemorialHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/memorial/entries", mh.handleEntries)
	mux.HandleFunc("/memorial/upload", mh.handleUpload)
	mux.HandleFunc("/memorial/canvas", mh.handleCanvas)
	mux.HandleFunc("/memorial/canvas/pixels", mh.handleCanvasPixels)
	mux.HandleFunc("/memorial/season", mh.handleSeason)
	mux.HandleFunc("/memorial/ws", mh.handleWebSocket)
}

// handleEntries returns memorial entries with optional filtering
func (mh *MemorialHandler) handleEntries(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		entryType := r.URL.Query().Get("type")
		season := r.URL.Query().Get("season")
		limitStr := r.URL.Query().Get("limit")

		limit := 50
		if l, err := fmt.Sscanf(limitStr, "%d", &limit); err == nil && l > 0 && limit > 100 {
			limit = 100
		}

		entries, err := mh.store.GetMemorialEntries(entryType, season, limit)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"entries": entries,
			"count":   len(entries),
		})
		return
	}

	if r.Method == http.MethodPost {
		var req struct {
			EntryType  string `json:"entry_type"`
			AuthorName string `json:"author_name"`
			AuthorEmail string `json:"author_email"`
			Content    string `json:"content"`
			SeasonTag  string `json:"season_tag"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		id := uuid.New().String()
		err := mh.store.SaveMemorialEntry(id, req.EntryType, req.AuthorName, req.AuthorEmail,
			req.Content, "", "", req.SeasonTag)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if mh.natsBus != nil {
			go mh.natsBus.PublishMemorial("entry_created", "anonymous", "text_entry", map[string]interface{}{
				"entry_id": id,
				"type":     req.EntryType,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":      id,
			"status":  "created",
		})
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// handleUpload handles file uploads for memorial
func (mh *MemorialHandler) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form
	r.ParseMultipartForm(int64(mh.cfg.UploadMaxSizeMB) * 1024 * 1024)

	file, handler, err := r.FormFile("media")
	if err != nil {
		http.Error(w, "No file provided", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Validate file type
	contentType := handler.Header.Get("Content-Type")
	allowed := false
	for _, t := range mh.cfg.AllowedTypes {
		if strings.HasPrefix(contentType, t) {
			allowed = true
			break
		}
	}
	if !allowed {
		http.Error(w, "File type not allowed", http.StatusBadRequest)
		return
	}

	// Save file
	uploadDir := "./uploads/memorial"
	os.MkdirAll(uploadDir, 0755)

	filename := fmt.Sprintf("%s_%s", uuid.New().String(), handler.Filename)
	filepath := filepath.Join(uploadDir, filename)

	out, err := os.Create(filepath)
	if err != nil {
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}
	defer out.Close()

	_, err = io.Copy(out, file)
	if err != nil {
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}

	// Save entry to database
	id := uuid.New().String()
	entryType := "image"
	if strings.HasPrefix(contentType, "video") {
		entryType = "video"
	}

	season := r.FormValue("season_tag")
	if season == "" {
		season = getCurrentSeason()
	}

	mh.store.SaveMemorialEntry(id, entryType, r.FormValue("author_name"),
		r.FormValue("author_email"), r.FormValue("content"),
		"/uploads/memorial/"+filename, contentType, season)

	metrics.RecordMemorialUpload(entryType)

	if mh.natsBus != nil {
		go mh.natsBus.PublishMemorial("upload_created", "anonymous", "file_upload", map[string]interface{}{
			"entry_id": id,
			"type":     entryType,
			"size":     handler.Size,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":       id,
		"filename": filename,
		"url":      "/uploads/memorial/" + filename,
		"type":     entryType,
		"season":   season,
	})
}

// handleCanvas handles collaborative canvas operations
func (mh *MemorialHandler) handleCanvas(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		pixels, err := mh.store.GetCanvasPixels()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"pixels": pixels,
			"count":  len(pixels),
		})
		return
	}

	if r.Method == http.MethodPost {
		var req struct {
			X      int    `json:"x"`
			Y      int    `json:"y"`
			Color  string `json:"color"`
			Author string `json:"author"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		if err := mh.store.SaveCanvasPixel(req.X, req.Y, req.Color, req.Author); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		metrics.RecordCanvasPixel()

		// Broadcast to all WebSocket clients
		msg, _ := json.Marshal(map[string]interface{}{
			"type":   "pixel",
			"x":      req.X,
			"y":      req.Y,
			"color":  req.Color,
			"author": req.Author,
		})
		mh.broadcast <- msg

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// handleCanvasPixels returns canvas pixels
func (mh *MemorialHandler) handleCanvasPixels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pixels, err := mh.store.GetCanvasPixels()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pixels)
}

// handleSeason returns current season info
func (mh *MemorialHandler) handleSeason(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	season := getCurrentSeason()
	if override := r.URL.Query().Get("override"); override != "" {
		season = override
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"season":        season,
		"auto_detected": r.URL.Query().Get("override") == "",
		"effects":       getSeasonEffects(season),
	})
}

// handleWebSocket handles WebSocket connections for real-time chat
func (mh *MemorialHandler) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	if !mh.cfg.WebSocketEnabled {
		http.Error(w, "WebSocket disabled", http.StatusServiceUnavailable)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error().Err(err).Msg("websocket upgrade failed")
		return
	}
	defer conn.Close()

	mh.clients[conn] = true
	metrics.UpdateWebSocketConnections(len(mh.clients))

	// Send welcome message
	conn.WriteJSON(map[string]interface{}{
		"type":    "welcome",
		"message": "Welcome to the Jackie Memorial",
		"season":  getCurrentSeason(),
	})

	for {
		var msg map[string]interface{}
		if err := conn.ReadJSON(&msg); err != nil {
			delete(mh.clients, conn)
			metrics.UpdateWebSocketConnections(len(mh.clients))
			break
		}

		// Broadcast message to all clients
		msg["timestamp"] = time.Now().Format(time.RFC3339)
		data, _ := json.Marshal(msg)
		mh.broadcast <- data

		if mh.natsBus != nil {
			go mh.natsBus.PublishMemorial("chat_message", msg["author"].(string), "chat", msg)
		}
	}
}

// handleBroadcasts sends messages to all connected clients
func (mh *MemorialHandler) handleBroadcasts() {
	for msg := range mh.broadcast {
		for client := range mh.clients {
			if err := client.WriteMessage(websocket.TextMessage, msg); err != nil {
				client.Close()
				delete(mh.clients, client)
				metrics.UpdateWebSocketConnections(len(mh.clients))
			}
		}
	}
}

// getCurrentSeason returns the current season based on date
func getCurrentSeason() string {
	month := time.Now().Month()
	switch {
	case month >= 3 && month <= 5:
		return "spring"
	case month >= 6 && month <= 8:
		return "summer"
	case month >= 9 && month <= 11:
		return "fall"
	default:
		return "winter"
	}
}

// getSeasonEffects returns effects for a season
func getSeasonEffects(season string) []string {
	switch season {
	case "spring":
		return []string{"bloom", "petals", "green_glow"}
	case "summer":
		return []string{"sun_glow", "fireflies", "warm_light"}
	case "fall":
		return []string{"falling_leaves", "orange_glow", "crisp_air"}
	case "winter":
		return []string{"snow", "frost", "cool_blue"}
	default:
		return []string{}
	}
}
