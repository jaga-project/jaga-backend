package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/jaga-project/jaga-backend/internal/database"
)

func (s *Server) handleCreateAdmin() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        var admin database.Admin
        if err := json.NewDecoder(r.Body).Decode(&admin); err != nil {
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        }
        // UserID seharusnya sudah ada dari request body (misalnya, admin baru adalah user yang sudah ada)
        // Atau jika UserID juga UUID yang di-generate di sini, pastikan unik dan sesuai.
        // Untuk CreatedAt, kita set di sini.
        admin.CreatedAt = time.Now()

        if err := database.CreateAdmin(r.Context(), s.db.Get(), &admin); err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        w.WriteHeader(http.StatusCreated)
        json.NewEncoder(w).Encode(admin)
    }
}

func (s *Server) handleGetAdmin() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        userID := mux.Vars(r)["user_id"] // Mengambil user_id dari path
        if userID != "" {
            admin, err := database.GetAdminByUserID(r.Context(), s.db.Get(), userID)
            if err != nil {
                http.Error(w, err.Error(), http.StatusNotFound)
                return
            }
            json.NewEncoder(w).Encode(admin)
            return
        }

        // Jika tidak ada user_id di path, list semua admin
        admins, err := database.ListAdmin(r.Context(), s.db.Get())
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        json.NewEncoder(w).Encode(admins)
    }
}

func (s *Server) handleUpdateAdmin() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        userID := mux.Vars(r)["user_id"]
        var adminUpdates database.Admin
        if err := json.NewDecoder(r.Body).Decode(&adminUpdates); err != nil {
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        }
        // Anda mungkin ingin memastikan UserID dari body sama dengan dari path,
        // atau hanya menggunakan UserID dari path dan mengabaikan dari body.
        // Untuk CreatedAt, bisa diupdate atau dibiarkan (tergantung kebutuhan).
        // Jika ingin CreatedAt tidak berubah, jangan sertakan di struct adminUpdates atau set nil.
        // Jika ingin update CreatedAt ke waktu sekarang:
        adminUpdates.CreatedAt = time.Now()


        if err := database.UpdateAdmin(r.Context(), s.db.Get(), userID, &adminUpdates); err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        w.WriteHeader(http.StatusOK)
        // Anda bisa memilih untuk mengirim kembali data admin yang sudah diupdate
        // updatedAdmin, _ := database.GetAdminByUserID(r.Context(), s.db.Get(), userID)
        // json.NewEncoder(w).Encode(updatedAdmin)
    }
}

func (s *Server) handleDeleteAdmin() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        userID := mux.Vars(r)["user_id"]
        if err := database.DeleteAdmin(r.Context(), s.db.Get(), userID); err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        w.WriteHeader(http.StatusNoContent)
    }
}

// RegisterAdminRoutes registers all admin-related routes
func (s *Server) RegisterAdminRoutes(r *mux.Router) {
    r.HandleFunc("/admins", s.handleCreateAdmin()).Methods("POST")
    r.HandleFunc("/admins", s.handleGetAdmin()).Methods("GET")           // List all admins
    r.HandleFunc("/admins/{user_id}", s.handleGetAdmin()).Methods("GET") // Get admin by user_id
    r.HandleFunc("/admins/{user_id}", s.handleUpdateAdmin()).Methods("PUT")
    r.HandleFunc("/admins/{user_id}", s.handleDeleteAdmin()).Methods("DELETE")
}