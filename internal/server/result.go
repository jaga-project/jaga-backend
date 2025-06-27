package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/jaga-project/jaga-backend/internal/database"
	"github.com/jaga-project/jaga-backend/internal/middleware"
)

// Structs untuk respons JSON
type CameraInfoResult struct {
    CameraID  int64   `json:"camera_id"`
    Name      string  `json:"name"`
    Latitude  float64 `json:"latitude"`
    Longitude float64 `json:"longitude"`
}

type SuspectInfo struct {
    SuspectID         int64            `json:"suspect_id"`
    EvidenceImageURL  *string          `json:"evidence_image_url,omitempty"`
    TimestampDetected time.Time        `json:"timestamp_detected"`
    PersonScore       float64          `json:"person_score"`
    MotorScore        float64          `json:"motor_score"`
    FinalScore        float64          `json:"final_score"`
    Camera            CameraInfoResult `json:"camera"`
}

type ResultResponse struct {
    LostReportID   int           `json:"lost_report_id"`
    AnalysisStatus string        `json:"analysis_status"`
    Suspects       []SuspectInfo `json:"suspects"`
}

func (s *Server) handleGetResultByLostReportID() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        idStr := mux.Vars(r)["id"]
        lostReportID, err := strconv.Atoi(idStr)
        if err != nil {
            writeJSONError(w, "invalid lost_report_id: must be an integer", http.StatusBadRequest)
            return
        }

        // --- OTORISASI ---
        requestingUserID, _ := r.Context().Value(middleware.UserIDContextKey).(string)
        isAdmin, _ := r.Context().Value(middleware.AdminStatusContextKey).(bool)

        report, err := database.GetLostReportWithVehicleInfoByID(r.Context(), s.db.Get(), lostReportID)
        if err != nil {
            if strings.Contains(err.Error(), "not found") {
                writeJSONError(w, "Lost report not found", http.StatusNotFound)
            } else {
                writeJSONError(w, "Failed to get lost report for authorization: "+err.Error(), http.StatusInternalServerError)
            }
            return
        }

        if !isAdmin && report.UserID != requestingUserID {
            writeJSONError(w, "Forbidden: You can only view results for your own reports.", http.StatusForbidden)
            return
        }
        // --- AKHIR OTORISASI ---

        suspectsFromDB, err := database.GetSuspectsByLostReportID(r.Context(), s.db.Get(), lostReportID)
        if err != nil {
            writeJSONError(w, "Failed to retrieve analysis results: "+err.Error(), http.StatusInternalServerError)
            return
        }

        response := ResultResponse{
            LostReportID: lostReportID,
            Suspects:     []SuspectInfo{},
        }

        if len(suspectsFromDB) == 0 {
            response.AnalysisStatus = "Processing or No Suspects Found"
        } else {
            response.AnalysisStatus = "Completed"
            for _, dbSuspect := range suspectsFromDB {
                suspectInfo := SuspectInfo{
                    SuspectID:         dbSuspect.SuspectID,
                    TimestampDetected: dbSuspect.DetectedTimestamp,
                    PersonScore:       dbSuspect.PersonScore,
                    MotorScore:        dbSuspect.MotorScore,
                    FinalScore:        dbSuspect.FinalScore,
                    Camera: CameraInfoResult{
                        CameraID:  dbSuspect.CameraID,
                        Name:      dbSuspect.CameraName,
                        Latitude:  dbSuspect.CameraLatitude,
                        Longitude: dbSuspect.CameraLongitude,
                    },
                }
                if dbSuspect.EvidenceImagePath.Valid && dbSuspect.EvidenceImagePath.String != "" {
                    url := "/" + strings.TrimPrefix(dbSuspect.EvidenceImagePath.String, "/")
                    suspectInfo.EvidenceImageURL = &url
                }
                response.Suspects = append(response.Suspects, suspectInfo)
            }
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(response)
    }
}

func (s *Server) RegisterResultRoutes(r *mux.Router) {
    r.HandleFunc("/results/{id:[0-9]+}", s.handleGetResultByLostReportID()).Methods("GET")
}