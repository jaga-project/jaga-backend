package server

import (
    "encoding/json"
   
    "net/http"
   
)

type ErrorDetail struct {
    Message string `json:"message"`
}

type JSONErrorResponse struct {
    Error ErrorDetail `json:"error"`
}

func writeJSONError(w http.ResponseWriter, message string, statusCode int) {
    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    w.WriteHeader(statusCode)
    response := JSONErrorResponse{
        Error: ErrorDetail{Message: message},
    }
    json.NewEncoder(w).Encode(response)
}