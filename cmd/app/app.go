package main

import (
	// "encoding/base64"
	"flag"
	"log"
	"math/rand"
	"net/http"
	"os"

	"time"
	"github.com/neversi/media-server-pion/pkg/server"
)

func init() {
	rand.Seed(time.Now().UnixNano())

}

var logFile = flag.String("l", "ws.log", "file to log data")
var addr = flag.String("p", ":8189", "port with preceding ':' character")

func main() {
	flag.Parse()
	f, err := os.OpenFile(*logFile, os.O_RDWR | os.O_CREATE | os.O_APPEND, 0666)
	if err != nil {
		log.Fatal("error opening log file: %v", err)
	}
	defer f.Close()
	log.SetOutput(f)
	log.Println("Websocket server started")
	wss := server.NewWSServer()
	wss.SetupServer()
	log.Fatal(http.ListenAndServe(*addr, nil))
}
