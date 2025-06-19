package server

import (
    "encoding/json"
   
    "net/http"
   
)

// ErrorDetail adalah struktur untuk detail error dalam respons JSON.
type ErrorDetail struct {
    Message string `json:"message"`
    // Code    string `json:"code,omitempty"` // Opsional: kode error internal
}

// JSONErrorResponse adalah struktur untuk respons error JSON standar.
type JSONErrorResponse struct {
    Error ErrorDetail `json:"error"`
}

// writeJSONError adalah helper untuk mengirim respons error dalam format JSON.
func writeJSONError(w http.ResponseWriter, message string, statusCode int) {
    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    w.WriteHeader(statusCode)
    response := JSONErrorResponse{
        Error: ErrorDetail{Message: message},
    }
    json.NewEncoder(w).Encode(response)
}