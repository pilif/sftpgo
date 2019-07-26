package api

import (
	"net/http"

	"github.com/pilif/sftpgo/logger"
	"github.com/pilif/sftpgo/sftpd"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/render"
)

// GetHTTPRouter returns the configured HTTP router
func GetHTTPRouter() http.Handler {
	return router
}

func initializeRouter() {
	router = chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(logger.NewStructuredLogger(logger.GetLogger()))
	router.Use(middleware.Recoverer)

	router.NotFound(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sendAPIResponse(w, r, nil, "Not Found", http.StatusNotFound)
	}))

	router.MethodNotAllowed(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sendAPIResponse(w, r, nil, "Method not allowed", http.StatusMethodNotAllowed)
	}))

	router.Get(activeConnectionsPath, func(w http.ResponseWriter, r *http.Request) {
		render.JSON(w, r, sftpd.GetConnectionsStats())
	})

	router.Delete(activeConnectionsPath+"/{connectionID}", func(w http.ResponseWriter, r *http.Request) {
		connectionID := chi.URLParam(r, "connectionID")
		if connectionID == "" {
			sendAPIResponse(w, r, nil, "connectionID is mandatory", http.StatusBadRequest)
			return
		}
		if sftpd.CloseActiveConnection(connectionID) {
			sendAPIResponse(w, r, nil, "Connection closed", http.StatusOK)
		} else {
			sendAPIResponse(w, r, nil, "Not Found", http.StatusNotFound)
		}
	})

	router.Get(quotaScanPath, func(w http.ResponseWriter, r *http.Request) {
		getQuotaScans(w, r)
	})

	router.Post(quotaScanPath, func(w http.ResponseWriter, r *http.Request) {
		startQuotaScan(w, r)
	})

	router.Get(userPath, func(w http.ResponseWriter, r *http.Request) {
		getUsers(w, r)
	})

	router.Post(userPath, func(w http.ResponseWriter, r *http.Request) {
		addUser(w, r)
	})

	router.Get(userPath+"/{userID}", func(w http.ResponseWriter, r *http.Request) {
		getUserByID(w, r)
	})

	router.Put(userPath+"/{userID}", func(w http.ResponseWriter, r *http.Request) {
		updateUser(w, r)
	})

	router.Delete(userPath+"/{userID}", func(w http.ResponseWriter, r *http.Request) {
		deleteUser(w, r)
	})
}
