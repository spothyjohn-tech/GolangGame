package main

import "game/server"

func main() {
	srv := server.NewChatServer()
	srv.Start("8080")
}
