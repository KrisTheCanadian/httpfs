package serve

import (
	"encoding/json"
	"fmt"
	"httpfs/cli"
	"httpfs/request"
	"httpfs/response"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
)

func Serve(opts *cli.Options) {

	port := strconv.Itoa(opts.Port)

	listener, err := net.Listen("tcp4", ":"+port)
	if err != nil {
		log.Printf("failed to listen on " + port + ".")
	}
	defer func(listener net.Listener) {
		err := listener.Close()
		if err != nil {
			log.Print("Error closing the server listener.")
		}
	}(listener)

	fmt.Println("echo server is listening on", listener.Addr())
	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error occured during accept connection %v\n", err)
			continue
		}
		go handleConnection(conn, opts)
	}
}

func handleConnection(conn net.Conn, opts *cli.Options) {
	defer func(conn net.Conn) {
		err := conn.Close()
		if err != nil {
			log.Print("Error closing the connection with the client.")
		}
	}(conn)
	log.Println(fmt.Printf("new connection from " + conn.RemoteAddr().String() + ". \n"))
	//we can use io.Copy(conn, conn) but this function demonstrates read&write methods
	buf := make([]byte, 1024)
	for {
		_, re := conn.Read(buf)
		if re != nil {
			fmt.Println("read error: ", re)
			break
		}
		// DEBUG
		_, err := conn.Write([]byte("Message received."))
		if err != nil {
			fmt.Println(err)
		}

		req := request.Parse(string(buf))
		data := request.Handle(req, opts)
		jsonData, err := json.Marshal(data)
		if err != nil {
			panic("Data cannot be converted to json. Internal Error")
		}
		headers := response.NewResponseHeaders()
		responseString := response.SendNewResponse(http.StatusOK, req.Protocol, req.Version, headers, string(jsonData))
		_, err = conn.Write([]byte(responseString))
		if err != nil {
			panic("Cannot send response. Error Occurred")
		}
	}
}
