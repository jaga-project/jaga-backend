package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"
	"sync"

	"github.com/gorilla/mux"
	"github.com/jaga-project/jaga-backend/internal/database"
	"github.com/jaga-project/jaga-backend/internal/middleware"
)

func (s *Server) handleCreateSuspect() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var suspect database.Suspect
		if err := json.NewDecoder(r.Body).Decode(&suspect); err != nil {
			writeJSONError(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		suspect.CreatedAt = time.Now()
		if err := database.CreateSuspect(r.Context(), s.db.Get(), &suspect); err != nil {
			log.Printf("ERROR: Failed to create suspect: %v", err)
			writeJSONError(w, "Failed to create suspect", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(suspect)
	}
}

func (s *Server) handleCreateManySuspects() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        var suspects []*database.Suspect
        if err := json.NewDecoder(r.Body).Decode(&suspects); err != nil {
            writeJSONError(w, "Invalid request body: expected an array of suspects", http.StatusBadRequest)
            return
        }

        if len(suspects) == 0 {
            writeJSONError(w, "Request body must contain at least one suspect", http.StatusBadRequest)
            return
        }

        tx, err := s.db.Get().BeginTx(r.Context(), nil)
        if err != nil {
            log.Printf("ERROR: Failed to begin transaction for batch suspect creation: %v", err)
            writeJSONError(w, "Failed to start database transaction", http.StatusInternalServerError)
            return
        }
        defer tx.Rollback()

        var wg sync.WaitGroup
        errChan := make(chan error, len(suspects))
        now := time.Now()

        for _, suspect := range suspects {
            wg.Add(1)
            go func(sp *database.Suspect) {
                defer wg.Done()
                sp.CreatedAt = now
                if err := database.CreateSuspectTx(r.Context(), tx, sp); err != nil {
                    errChan <- err
                }
            }(suspect)
        }

        wg.Wait()
        close(errChan)

        for err := range errChan {
            if err != nil {
                log.Printf("ERROR: Failed to create a suspect in batch: %v", err)
                writeJSONError(w, fmt.Sprintf("Failed to create one or more suspects: %v", err), http.StatusInternalServerError)
                return
            }
        }

        if err := tx.Commit(); err != nil {
            log.Printf("ERROR: Failed to commit transaction for batch suspect creation: %v", err)
            writeJSONError(w, "Failed to commit transaction", http.StatusInternalServerError)
            return
        }

        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusCreated)
        json.NewEncoder(w).Encode(map[string]string{"message": fmt.Sprintf("%d suspects created successfully", len(suspects))})
    }
}

func (s *Server) handleListSuspects() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list, err := database.ListSuspects(r.Context(), s.db.Get())
		if err != nil {
			log.Printf("ERROR: Failed to list suspects: %v", err)
			writeJSONError(w, fmt.Sprintf("Failed to retrieve suspects: %v", err), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(list)
	}
}

func (s *Server) handleGetSuspectByID() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := mux.Vars(r)["id"]
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			writeJSONError(w, "Invalid suspect ID", http.StatusBadRequest)
			return
		}

		suspect, err := database.GetSuspectByID(r.Context(), s.db.Get(), id)
		if err != nil {
			if err.Error() == "suspect not found" {
				writeJSONError(w, err.Error(), http.StatusNotFound)
			} else {
				log.Printf("ERROR: Failed to get suspect by ID %d: %v", id, err)
				writeJSONError(w, "Failed to retrieve suspect", http.StatusInternalServerError)
			}
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(suspect)
	}
}

func (s *Server) handleUpdateSuspect() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := mux.Vars(r)["id"]
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			writeJSONError(w, "Invalid suspect ID", http.StatusBadRequest)
			return
		}
		var suspect database.Suspect
		if err := json.NewDecoder(r.Body).Decode(&suspect); err != nil {
			writeJSONError(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if err := database.UpdateSuspect(r.Context(), s.db.Get(), id, &suspect); err != nil {
			log.Printf("ERROR: Failed to update suspect %d: %v", id, err)
			writeJSONError(w, "Failed to update suspect", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"message": "Suspect updated successfully"})
	}
}

func (s *Server) handleDeleteSuspect() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := mux.Vars(r)["id"]
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			writeJSONError(w, "Invalid suspect ID", http.StatusBadRequest)
			return
		}
		if err := database.DeleteSuspect(r.Context(), s.db.Get(), id); err != nil {
			log.Printf("ERROR: Failed to delete suspect %d: %v", id, err)
			writeJSONError(w, "Failed to delete suspect", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func (s *Server) RegisterSuspectRoutes(r *mux.Router) {
	adminOnlyMiddleware := middleware.AdminOnlyMiddleware()

	r.Handle("/suspects", adminOnlyMiddleware(s.handleCreateSuspect())).Methods("POST")
	r.Handle("/suspects/batch", adminOnlyMiddleware(s.handleCreateManySuspects())).Methods("POST")
	r.Handle("/suspects", adminOnlyMiddleware(s.handleListSuspects())).Methods("GET")
	r.Handle("/suspects/{id:[0-9]+}", adminOnlyMiddleware(s.handleGetSuspectByID())).Methods("GET")
	r.Handle("/suspects/{id:[0-9]+}", adminOnlyMiddleware(s.handleUpdateSuspect())).Methods("PUT")
	r.Handle("/suspects/{id:[0-9]+}", adminOnlyMiddleware(s.handleDeleteSuspect())).Methods("DELETE")
}

