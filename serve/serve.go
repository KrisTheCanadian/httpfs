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
	"time"
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
	DATA   uint8 = 5
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
		go handlePackets(buf[:n], *addr, n)
	}
}

func handlePackets(buf []byte, addr net.UDPAddr, n int) {
	m, udpConn, err, bSynAckMessage, badPacket := initiateHandshake(buf, addr, n)
	if badPacket {
		return
	}

	// TODO Handle HTTP REQUEST FRAMES
	// TEMPORARY
	// Simple but not super efficient -> We wait until receiving an ACK before sending new frame.
	frames := make(map[int]message)
	err = udpConn.SetReadDeadline(time.Now().Add(10 * time.Second))
	checkError(err)
	for {
		buf := make([]byte, 1024)
		n, err := udpConn.Read(buf)
		if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
			err = udpConn.SetReadDeadline(time.Now().Add(10 * time.Second))
			// TODO ADD LOGIC TO CHECK IF THERE'S A MISSING SEQUENCE
			// TODO NACK THE MISSING ONES
			break
		}
		if err != nil {
			if e, ok := err.(net.Error); !ok || !e.Timeout() {
				continue
			}
		}
		// CHECK FOR DATA
		m = parseMessage(buf, n)
		if m.packetType != DATA {
			// ignore it and continue waiting
			continue
		}
		// check to see if the frame is already there first
		frames[int(m.sequenceNumber)] = m
		split := strings.Split(udpConn.RemoteAddr().String(), ":")
		ipAddress := split[0]

		port, err := strconv.Atoi(split[1])
		checkError(err)

		payloadBuffer := make([]byte, 1024)
		payloadBufferWriter := bytes.NewBuffer(payloadBuffer)
		payloadBufferWriter.WriteString("0")

		ackPacket := createMessage(ACK, int(m.sequenceNumber), ipAddress, port, payloadBufferWriter.Bytes())
		bAckPacket := convertMessageToBytes(ackPacket)

		// sending ACK
		_, err = udpConn.Write(bAckPacket.Bytes())
	}

	checkError(err)
	fmt.Println(bSynAckMessage)
	fmt.Println(m)
	fmt.Println(udpConn)
	//handleHTTP(udpConn, opts, buf, addr)
	// WAIT FOR MORE REQUESTS AND STUFF.
}

func initiateHandshake(buf []byte, addr net.UDPAddr, n int) (message, *net.UDPConn, error, bytes.Buffer, bool) {
	log.Println("UDP client : ", addr)
	log.Println("Received from UDP client :  ", buf[:n])

	fmt.Println(buf[:n])
	m := parseMessage(buf, n)

	// check if SYN
	if m.packetType != SYN {
		// DROP THE PACKET and wait for a correct one.
		return message{}, nil, nil, bytes.Buffer{}, true
	}

	// CREATE A NEW SOCKET
	udpConn, err := net.DialUDP("udp", nil, &addr)

	fmt.Println("Created Socket to Handle: " + udpConn.LocalAddr().String())
	fmt.Println("Handling Client connection from " + udpConn.RemoteAddr().String())

	// Send SYN/ACK
	payloadBuffer := make([]byte, 1024)
	payloadBufferWriter := bytes.NewBuffer(payloadBuffer)
	payloadBufferWriter.WriteString("0")

	synAckMessage := createMessage(SYNACK, 2, addr.IP.String(), addr.Port, payloadBufferWriter.Bytes())
	bSynAckMessage := convertMessageToBytes(synAckMessage)

	fmt.Println("Sending Client a " + addr.IP.String() + ":" + strconv.Itoa(addr.Port) + " a SYN/ACK Packet")

	// start timer and read the message
	_, err = udpConn.Write(bSynAckMessage.Bytes())
	err = udpConn.SetReadDeadline(time.Now().Add(10 * time.Second))
	checkError(err)
	for {
		buf := make([]byte, 1024)
		n, err := udpConn.Read(buf)
		if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
			// resend the SYN ACK Request
			_, err = udpConn.Write(bSynAckMessage.Bytes())
			fmt.Println("Attempting to resend SYN/ACK to " + addr.IP.String() + ":" + strconv.Itoa(addr.Port))
			err = udpConn.SetReadDeadline(time.Now().Add(5 * time.Second))
			continue
		}
		if err != nil {
			if e, ok := err.(net.Error); !ok || !e.Timeout() {
				continue
			}
		}
		fmt.Println(buf[:n])
		fmt.Println("Received a packet as a response! from " + addr.String())

		// CHECK FOR ACK
		m := parseMessage(buf, n)
		if m.packetType != ACK {
			// ignore it and continue waiting
			continue
		} else {
			fmt.Println("Handshake Completed.")
			break
		}
	}
	return m, udpConn, err, bSynAckMessage, false
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

func createMessage(packetType uint8, sequenceNumber int, host string, port int, payload []byte) encodedMessage {
	// Parse the address
	octets := strings.Split(host, ".")

	octet0, _ := strconv.Atoi(octets[0])
	octet1, _ := strconv.Atoi(octets[1])
	octet2, _ := strconv.Atoi(octets[2])
	octet3, _ := strconv.Atoi(octets[3])

	bAddress := [4]byte{byte(octet0), byte(octet1), byte(octet2), byte(octet3)}

	portBuffer := [2]byte{}
	binary.LittleEndian.PutUint16(portBuffer[:], uint16(port))

	sequenceNumberBuffer := [4]byte{}
	binary.BigEndian.PutUint32(sequenceNumberBuffer[:], uint32(sequenceNumber))

	// Start Handshake
	// SYN MESSAGE
	m := encodedMessage{packetType: [1]byte{packetType}, sequenceNumber: sequenceNumberBuffer, peerAddress: bAddress, peerPort: portBuffer, payload: payload}
	return m
}

func parseMessage(buf []byte, n int) message {
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
