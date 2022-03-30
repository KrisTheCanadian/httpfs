package serve

import (
	"bufio"
	"bytes"
	"encoding/binary"
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

type message struct {
	packetType     int
	sequenceNumber int
	peerAddress    string
	peerPort       int
	payload        []byte
}

func Serve(opts *cli.Options) {

	port := strconv.Itoa(opts.Port)
	address := "127.0.0.1:" + port
	raddr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		log.Fatal(err)
	}
	udpConn, err := net.ListenUDP("udp", raddr)
	if err != nil {
		log.Printf("failed to listen on " + port + ".")
	}
	defer func(udpConn net.PacketConn) {
		err := udpConn.Close()
		if err != nil {
			log.Print("Error closing the server.")
		}
	}(udpConn)

	fmt.Println("echo server is listening on", udpConn.LocalAddr().String())
	buf := make([]byte, 1024)

	for {
		n, addr, err := udpConn.ReadFromUDP(buf)
		if err != nil {
			continue
		}
		go handlePackets(opts, buf[:n], *addr, n)
	}
}

func handlePackets(opts *cli.Options, buf []byte, addr net.UDPAddr, n int) {
	log.Println("UDP client : ", addr)
	log.Println("Received from UDP client :  ", string(buf[:n]))

	// CREATE A NEW SOCKET
	udpConn, err := net.DialUDP("udp", nil, &addr)

	// HERE WE NEED A PORT THAT IS AVAILABLE
	raddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	checkError(err)

	udpListen, err := net.ListenUDP("udp", raddr)
	checkError(err)

	fmt.Println(string(buf[:n]))
	// CHECK FOR SYN
	frame := buf[:n]
	readerFrame := bytes.NewReader(frame)
	scanner := bufio.NewScanner(readerFrame)
	scanner.Split(bufio.ScanBytes)

	if !scanner.Scan() {
		fmt.Print("No response.")
		os.Exit(1)
	}

	m := message{}
	for scanner.Scan() {
		bPacketType := scanner.Bytes()
		fmt.Printf("%v = %v = %v\n", bPacketType, bPacketType[0], string(bPacketType))
		m.packetType = int(binary.LittleEndian.Uint64(bPacketType))
	}
	fmt.Println(m.packetType)
	fmt.Println(frame)
	fmt.Println(udpConn)
	fmt.Println(udpListen)
	//handleHTTP(udpConn, opts, buf, addr)
	// WAIT FOR MORE REQUESTS AND STUFF.
}

func checkError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func handleHTTP(udpConn *net.UDPConn, opts *cli.Options, buf []byte, addr net.UDPAddr) {
	req, err := request.Parse(string(buf))
	data, err := request.Handle(req, opts)
	if err != nil {
		httpError, _ := strconv.Atoi(err.Error())
		responseString := response.SendHTTPError(httpError, req.Protocol, req.Version)
		_, err = udpConn.WriteToUDP([]byte(responseString), &addr)
		return
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		responseString := response.SendHTTPError(http.StatusInternalServerError, req.Protocol, req.Version)
		_, err = udpConn.WriteToUDP([]byte(responseString), &addr)
		return
	}

	headers, stayConnected := response.NewResponseHeaders(req)
	responseString := response.SendNewResponse(http.StatusOK, req.Protocol, req.Version, headers, string(jsonData))
	_, err = udpConn.WriteToUDP([]byte(responseString), &addr)
	if err != nil {
		responseString := response.SendHTTPError(http.StatusInternalServerError, req.Protocol, req.Version)
		_, err = udpConn.WriteToUDP([]byte(responseString), &addr)
		return
	}
	if !stayConnected {
		return
	}
}
