package main

import "game/client"

func main() {
	cl := client.NewChatClient("http://localhost:8080")
	cl.Start()
}