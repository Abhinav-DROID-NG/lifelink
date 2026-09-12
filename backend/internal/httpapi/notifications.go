package httpapi

import (
	"net/http"
	"strconv"
)

func (s *Server) handleListNotifications(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())

	notifs, err := s.store.ListNotifications(r.Context(), user.ID, 50)
	if err != nil {
		s.logger.Error("list notifications", "err", err)
		writeError(w, ErrInternal)
		return
	}
	unread, err := s.store.UnreadNotificationCount(r.Context(), user.ID)
	if err != nil {
		s.logger.Error("unread count", "err", err)
		writeError(w, ErrInternal)
		return
	}
	writeSuccess(w, http.StatusOK, map[string]any{
		"items":        notifs,
		"unread_count": unread,
	})
}

func (s *Server) handleMarkNotificationRead(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, ErrBadRequest)
		return
	}
	if err := s.store.MarkNotificationRead(r.Context(), user.ID, id); err != nil {
		writeNotFound(w, "Notification")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleReadAllNotifications(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())
	if err := s.store.MarkAllNotificationsRead(r.Context(), user.ID); err != nil {
		s.logger.Error("mark all read", "err", err)
		writeError(w, ErrInternal)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
