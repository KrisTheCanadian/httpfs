package serve

import (
	"fmt"
	"httpfs/cli"
	"httpfs/request"
	"io"
	"log"
	"net"
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
		n, re := conn.Read(buf)
		if n > 0 {
			if _, we := conn.Write(buf[:n]); we != nil {
				fmt.Println("write error: ", we)
				break
			}
		}
		if re == io.EOF {
			break
		}
		if re != nil {
			fmt.Println("read error: ", re)
			break
		}
		parsed := request.Parse(string(buf))
		request.Handle(parsed, opts)
	}
}
