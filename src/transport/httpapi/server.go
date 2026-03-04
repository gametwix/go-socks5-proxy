package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"socks5-proxy/src/domain"
)

type UserService interface {
	Create(ctx context.Context, username, password string, enabled bool) error
	Update(ctx context.Context, username string, password *string, enabled *bool) error
	Delete(ctx context.Context, username string) error
	Get(ctx context.Context, username string) (domain.User, error)
	List(ctx context.Context) ([]domain.User, error)
}

type StatsService interface {
	Get(ctx context.Context, username string) (domain.UserStats, error)
	List(ctx context.Context) ([]domain.UserStats, error)
}

type Server struct {
	users UserService
	stats StatsService
}

type createUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Enabled  *bool  `json:"enabled,omitempty"`
}

type updateUserRequest struct {
	Password *string `json:"password,omitempty"`
	Enabled  *bool   `json:"enabled,omitempty"`
}

func New(users UserService, stats StatsService) *Server {
	return &Server{users: users, stats: stats}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.health)
	mux.HandleFunc("/users", s.usersHandler)
	mux.HandleFunc("/users/", s.userByNameHandler)
	mux.HandleFunc("/stats", s.statsHandler)
	mux.HandleFunc("/stats/", s.statsByNameHandler)
	return mux
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) usersHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	switch r.Method {
	case http.MethodGet:
		users, err := s.users.List(ctx)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, users)
	case http.MethodPost:
		var req createUserRequest
		if err := decodeJSON(r, &req); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		enabled := true
		if req.Enabled != nil {
			enabled = *req.Enabled
		}
		if err := s.users.Create(ctx, req.Username, req.Password, enabled); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		u, err := s.users.Get(ctx, req.Username)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, u)
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) userByNameHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username := strings.TrimPrefix(r.URL.Path, "/users/")
	if username == "" {
		writeErr(w, http.StatusBadRequest, "username is required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		u, err := s.users.Get(ctx, username)
		if err != nil {
			if errors.Is(err, domain.ErrUserNotFound) {
				writeErr(w, http.StatusNotFound, "user not found")
				return
			}
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, u)
	case http.MethodPut:
		var req updateUserRequest
		if err := decodeJSON(r, &req); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		if req.Password == nil && req.Enabled == nil {
			writeErr(w, http.StatusBadRequest, "no fields to update")
			return
		}
		if err := s.users.Update(ctx, username, req.Password, req.Enabled); err != nil {
			if errors.Is(err, domain.ErrUserNotFound) {
				writeErr(w, http.StatusNotFound, "user not found")
				return
			}
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		u, err := s.users.Get(ctx, username)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, u)
	case http.MethodDelete:
		if err := s.users.Delete(ctx, username); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) statsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	stats, err := s.stats.List(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (s *Server) statsByNameHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	username := strings.TrimPrefix(r.URL.Path, "/stats/")
	if username == "" {
		writeErr(w, http.StatusBadRequest, "username is required")
		return
	}
	stats, err := s.stats.Get(r.Context(), username)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func decodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	return nil
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
