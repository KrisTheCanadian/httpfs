package serve

import (
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
	"strconv"
	"strings"
)

type encodedMessage struct {
	packetType     [1]byte
	sequenceNumber [4]byte
	peerAddress    [4]byte
	peerPort       [2]byte
	// Size 1013
	payload []byte
}

type message struct {
	packetType     uint8
	sequenceNumber uint32
	peerAddress    string
	peerPort       uint16
	payload        string
}

const (
	ACK    uint8 = 0
	SYN    uint8 = 1
	FIN    uint8 = 2
	NACK   uint8 = 3
	SYNACK uint8 = 4
)

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
	udpAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	checkError(err)

	udpListen, err := net.ListenUDP("udp", udpAddr)
	checkError(err)

	fmt.Println(buf[:n])
	m := parseMessage(buf, err, n)

	// check if SYN
	if m.packetType != SYN {
		// DROP THE PACKET and wait for a correct one.
	}

	// Send SYN/ACK
	synAckMessage := createMessage(SYNACK, m.peerAddress)
	bSynAckMessage := convertMessageToBytes(synAckMessage)

	_, err = udpConn.WriteToUDP(bSynAckMessage.Bytes(), &addr)
	checkError(err)
	fmt.Println(m)
	fmt.Println(udpConn)
	fmt.Println(udpListen)
	//handleHTTP(udpConn, opts, buf, addr)
	// WAIT FOR MORE REQUESTS AND STUFF.
}

func convertMessageToBytes(m encodedMessage) bytes.Buffer {
	// create the message buffer
	var bMessage bytes.Buffer
	bMessage.Write(m.packetType[:])
	bMessage.Write(m.sequenceNumber[:])
	bMessage.Write(m.peerAddress[:])
	bMessage.Write(m.peerPort[:])
	bMessage.Write(m.payload)
	return bMessage
}

func createMessage(packetType uint8, host string) encodedMessage {
	// Parse the address
	octets := strings.Split(host, ".")

	octet0, _ := strconv.Atoi(octets[0])
	octet1, _ := strconv.Atoi(octets[1])
	octet2, _ := strconv.Atoi(octets[2])
	octet3, _ := strconv.Atoi(octets[3])

	bAddress := [4]byte{byte(octet0), byte(octet1), byte(octet2), byte(octet3)}

	fmt.Printf("%s has 4-byte representation of %bAddress\n", host, bAddress)

	portBuffer := [2]byte{}
	binary.LittleEndian.PutUint16(portBuffer[:], 8080)

	sequenceNumberBuffer := [4]byte{}
	binary.BigEndian.PutUint32(sequenceNumberBuffer[:], 0)

	// Start Handshake
	// SYN MESSAGE
	m := encodedMessage{packetType: [1]byte{packetType}, sequenceNumber: sequenceNumberBuffer, peerAddress: bAddress, peerPort: portBuffer, payload: []byte("0")}
	return m
}

func parseMessage(buf []byte, err error, n int) message {
	// Parse the address
	bAddressOctet0 := buf[5]
	bAddressOctet1 := buf[6]
	bAddressOctet2 := buf[7]
	bAddressOctet3 := buf[8]

	addressInt0 := strconv.Itoa(int(bAddressOctet0))
	addressInt1 := strconv.Itoa(int(bAddressOctet1))
	addressInt2 := strconv.Itoa(int(bAddressOctet2))
	addressInt3 := strconv.Itoa(int(bAddressOctet3))

	host := addressInt0 + "." + addressInt1 + "." + addressInt2 + "." + addressInt3

	port := binary.LittleEndian.Uint16(buf[9:11])
	checkError(err)

	m := message{}
	m.packetType = buf[0]
	m.sequenceNumber = binary.BigEndian.Uint32(buf[1:5])
	m.peerAddress = host
	m.peerPort = port
	m.payload = string(buf[11:n])
	return m
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
