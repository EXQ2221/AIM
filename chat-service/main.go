package main

import (
	chat "example.com/aim/chat-service/kitex_gen/chat/chatservice"
	"log"
)

func main() {
	svr := chat.NewServer(new(ChatServiceImpl))

	err := svr.Run()

	if err != nil {
		log.Println(err.Error())
	}
}
