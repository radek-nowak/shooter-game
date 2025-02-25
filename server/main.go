// package main
//
// import (
// 	"bufio"
// 	"fmt"
// 	"log"
// 	"net"
// 	"os"
// 	"sync"
// )
//
// const ServerPort = ":8080"
//
// // Server side code
// func startServer() {
// 	listener, err := net.Listen("tcp", ServerPort)
// 	if err != nil {
// 		log.Fatal("Failed to start server:", err)
// 	}
// 	defer listener.Close()
// 	log.Println("Server started on", ServerPort)
//
// 	clients := make(map[net.Conn]bool)
// 	var mu sync.Mutex
//
// 	for {
// 		conn, err := listener.Accept()
// 		if err != nil {
// 			log.Println("Connection error:", err)
// 			continue
// 		}
//
// 		mu.Lock()
// 		clients[conn] = true
// 		mu.Unlock()
//
// 		go func(c net.Conn) {
// 			reader := bufio.NewReader(c)
// 			for {
// 				msg, err := reader.ReadString('\n')
// 				if err != nil {
// 					log.Println("Client disconnected:", err)
// 					mu.Lock()
// 					delete(clients, c)
// 					mu.Unlock()
// 					return
// 				}
//
// 				mu.Lock()
// 				for client := range clients {
// 					if client != c {
// 						client.Write([]byte(msg))
// 					}
// 				}
// 				mu.Unlock()
// 			}
// 		}(conn)
// 	}
// }
//
// func main() {
// 	if len(os.Args) > 1 && os.Args[1] == "server" {
// 		startServer()
// 		return
// 	}
//
// 	if len(os.Args) < 3 {
// 		fmt.Println("Usage: go run main.go <player_id> <server_ip:port>")
// 		return
// 	}
//
// 	playerID := os.Args[1]
// 	serverAddr := os.Args[2]
//
// 	conn, err := net.Dial("tcp", serverAddr)
// 	if err != nil {
// 		log.Fatal("Failed to connect to server:", err)
// 	}
// 	defer conn.Close()
//
// 	game := &Game{
// 		player:  NewPlayer(playerID, float64(ScreenWidth)/2, float64(ScreenHeight)/2),
// 		players: make(map[string]*Player),
// 		conn:    conn,
// 	}
//
// 	go game.listenForUpdates()
//
// 	ebiten.SetWindowSize(ScreenWidth, ScreenHeight)
// 	ebiten.SetWindowTitle("2D Multiplayer Top-Down Shooter")
// 	if err := ebiten.RunGame(game); err != nil {
// 		log.Fatal(err)
// 	}
// }
