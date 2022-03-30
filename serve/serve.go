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
	"strconv"
	"time"
)

func Serve(opts *cli.Options) {

	port := strconv.Itoa(opts.Port)
	address := "127.0.0.1:" + port
	//raddr, err := net.ResolveUDPAddr("udp", address)
	//if err != nil {
	//	log.Fatal(err)
	//}
	packetConn, err := net.ListenPacket("udp", address)
	if err != nil {
		log.Printf("failed to listen on " + port + ".")
	}
	defer func(udpConn net.PacketConn) {
		err := udpConn.Close()
		if err != nil {
			log.Print("Error closing the server.")
		}
	}(packetConn)

	fmt.Println("echo server is listening on", packetConn.LocalAddr().String())
	for {
		buf := make([]byte, 1024)
		n, addr, err := packetConn.ReadFrom(buf)
		if err != nil {
			continue
		}
		go handlePackets(packetConn, opts, buf[:n], addr, n)
	}

}

func handlePackets(packetConn net.PacketConn, opts *cli.Options, buf []byte, addr net.Addr, n int) {
	defer func(packetConn net.PacketConn) {
		err := packetConn.Close()
		if err != nil {
			log.Print("Error closing the connection with the client.")
		}
	}(packetConn)

	for {
		fmt.Println("UDP client : ", addr)
		fmt.Println("Received from UDP client :  ", string(buf[:n]))
		// we are adding a 3-second deadline for the packet to be written.
		deadline := time.Now().Add(time.Second * 3)
		err := packetConn.SetWriteDeadline(deadline)

		req, err := request.Parse(string(buf))
		data, err := request.Handle(req, opts)
		if err != nil {
			httpError, _ := strconv.Atoi(err.Error())
			responseString := response.SendHTTPError(httpError, req.Protocol, req.Version)
			_, err = packetConn.WriteTo([]byte(responseString), addr)
			break
		}
		// TODO BEAUTIFY THE JSON
		jsonData, err := json.Marshal(data)
		if err != nil {
			responseString := response.SendHTTPError(http.StatusInternalServerError, req.Protocol, req.Version)
			_, err = packetConn.WriteTo([]byte(responseString), addr)
			break
		}

		headers, stayConnected := response.NewResponseHeaders(req)
		responseString := response.SendNewResponse(http.StatusOK, req.Protocol, req.Version, headers, string(jsonData))
		_, err = packetConn.WriteTo([]byte(responseString), addr)
		if err != nil {
			responseString := response.SendHTTPError(http.StatusInternalServerError, req.Protocol, req.Version)
			_, err = packetConn.WriteTo([]byte(responseString), addr)
			break
		}
		if !stayConnected {
			break
		}
	}
}
