package main

import (
	"fmt"
    "github.com/jaga-project/jaga-backend/internal/server"
)

func main() {
	server := server.NewServer()
	fmt.Printf("JAGA Backend Starting ...\n\nStarting server on %s\n", server.Addr)
	err := server.ListenAndServe()
	if err != nil {
		panic(fmt.Sprintf("cannot start server: %s", err))
	}
}