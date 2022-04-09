package serve

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"httpfs/cli"
	"httpfs/request"
	"httpfs/response"
	"log"
	"net"
	"net/http"
	"sort"
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
	FINACK uint8 = 6
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

	log.Println("echo server is listening on", udpConn.LocalAddr().String())
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
	udpConn, err, _, badPacket, port := initiateHandshake(buf, addr, n)
	if badPacket {
		return
	}

	// Handling the request
	reqFrames := handleRequestFrames(udpConn, port)
	sbPayload := parseFrames(reqFrames)
	if sbPayload.String() == "" {
		reqFrames = handleRequestFrames(udpConn, port)
		sbPayload = parseFrames(reqFrames)
	}
	log.Println("Payload Completely Received: \n" + sbPayload.String())

	// Let's create the response
	res := handleHTTP(opts, sbPayload.String())

	// Sending the response (Also handles FIN)
	sendResponse(res, udpConn, err, port)

	// Send FIN/ACK
	sendPacket(udpConn, port, FINACK, 70)
	log.Println("Sent FIN/ACK Packet!")

	// Handle ACK
	handleLastAck(udpConn, ACK)

	log.Println("Completed!")
}

func handleLastAck(udpConn *net.UDPConn, packetType uint8) {
	// START WAITING FOR THAT ACK FRAME
	err := udpConn.SetReadDeadline(time.Now().Add(4 * time.Second))
	checkError(err)
	count := 5
	for {
		buf := make([]byte, 1024)
		n, addr, err := udpConn.ReadFromUDP(buf)
		if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
			err = udpConn.SetReadDeadline(time.Now().Add(4 * time.Second))
			count--
			if count < 0 {
				break
			}
			continue
		}
		if err != nil {
			if e, ok := err.(net.Error); !ok || !e.Timeout() {
				err = udpConn.SetReadDeadline(time.Now().Add(4 * time.Second))
				continue
			}
		}

		// CHECK FOR ACK
		m := parseMessage(buf, n)

		log.Println("Received a " + strconv.Itoa(int(m.packetType)) + " packet as a response! from " + addr.String())
		err = udpConn.SetReadDeadline(time.Now().Add(4 * time.Second))

		if m.packetType == DATA {
			break
		}

		if m.packetType != packetType {
			// ignore it and continue waiting
			continue
		}
		// CHECK IF SEQUENCE MATCHES A PACKET SENT
		frameNumber := int(m.sequenceNumber)
		log.Println("Ack packet received for frame:" + strconv.Itoa(frameNumber))
		break
	}
}

func sendPacket(udpConn *net.UDPConn, port string, packetType uint8, frameNumber int) (map[int]encodedMessage, bytes.Buffer, map[int]bool, string) {
	// SEND THE MESSAGE FRAMES
	// encode the FIN Packet
	ipAddress, _ := connectionGetPortAndIP(udpConn)
	ackedFrames := make(map[int]bool)
	frames := make(map[int]encodedMessage)
	payloadBuffer := make([]byte, 50)
	payloadBufferWriter := bytes.NewBuffer(payloadBuffer)
	payloadBufferWriter.WriteString("0")
	sPort, _ := strconv.Atoi(port)
	m := createMessage(packetType, frameNumber, ipAddress, sPort, payloadBufferWriter.Bytes())
	frames[frameNumber] = m
	messageToBytes := convertMessageToBytes(m)
	// send it to the client
	_, err := udpConn.Write(messageToBytes.Bytes())
	checkError(err)
	ackedFrames = make(map[int]bool)
	ackedFrames[frameNumber] = false
	return frames, messageToBytes, ackedFrames, ipAddress
}

func sendResponse(res string, udpConn *net.UDPConn, err error, port string) {
	bRes := []byte(res)

	resFrames := make(map[int]encodedMessage)
	ackedFrames := make(map[int]bool)

	chunkDataIntoFrames(bRes, udpConn, resFrames, ackedFrames, port)

	sendFrames(resFrames, udpConn, ackedFrames)

	// START WAITING FOR THOSE ACK FRAMES
	count := 10
	// CHECK FOR FIN PACKET
	err = udpConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	checkError(err)
	for {
		buf := make([]byte, 1024)
		n, addr, err := udpConn.ReadFromUDP(buf)
		if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
			isAllAcked := true
			// CHECK ACKED Frames
			for frameNumber, isAcked := range ackedFrames {
				isAllAcked = isAllAcked && isAcked
				if !isAcked {
					log.Println("Sending Packet: " + strconv.Itoa(frameNumber))
					frameMessage := resFrames[frameNumber]
					bMessage := convertMessageToBytes(frameMessage)
					_, err = udpConn.Write(bMessage.Bytes())
					checkError(err)
				}
			}
			if isAllAcked {
				// ALL PACKETS HAVE BEEN ACKED.
				log.Println("Packets all acked... Waiting for FIN Packet... ")
				err = udpConn.SetReadDeadline(time.Now().Add(5 * time.Second))
				continue
			}
			// RESET TIMER
			err = udpConn.SetReadDeadline(time.Now().Add(5 * time.Second))
			count--
			if count < 0 {
				break
			}
			continue
		}
		if err != nil {
			if e, ok := err.(net.Error); !ok || !e.Timeout() {
				continue
			}
		}

		m := parseMessage(buf, n)

		log.Println("Received a " + strconv.Itoa(int(m.packetType)) + " packet as a response! from " + addr.String())

		// CHECK FOR FIN -> client has most likely received all packets
		// some acked packets from the client was most likely dropped.
		if m.packetType == FIN {
			break
		}

		if m.packetType != ACK {
			// ignore it and continue waiting
			continue
		}
		// CHECK IF SEQUENCE MATCHES A PACKET SENT
		frameNumber := int(m.sequenceNumber)
		if ackedFrames[frameNumber] {
			// We already received this
			continue
		}
		log.Println("Ack packet received for frame:" + strconv.Itoa(frameNumber))
		ackedFrames[frameNumber] = true
		err = udpConn.SetReadDeadline(time.Now().Add(6 * time.Second))
	}
}

func sendFrames(resFrames map[int]encodedMessage, udpConn *net.UDPConn, ackedFrames map[int]bool) {
	// SEND THE MESSAGE FRAMES
	for number, frameMessage := range resFrames {
		// encode that frame
		bMessage := convertMessageToBytes(frameMessage)
		// send it to the client
		_, err := udpConn.Write(bMessage.Bytes())
		checkError(err)
		ackedFrames[number] = false
		log.Println("Sent frame: " + strconv.Itoa(number))
	}
	log.Println("Sent all frames!")
}

func chunkDataIntoFrames(bRes []byte, udpConn *net.UDPConn, resFrames map[int]encodedMessage, ackedFrames map[int]bool, port string) {
	chunks := split(bRes, 1013)
	frameNumber := 60

	// SPLIT PAYLOAD INTO FRAMES
	for _, chunk := range chunks {
		ipAddress, _ := connectionGetPortAndIP(udpConn)
		intPort, _ := strconv.Atoi(port)
		m := createMessage(DATA, frameNumber, ipAddress, intPort, chunk)
		resFrames[frameNumber] = m
		ackedFrames[frameNumber] = false
		frameNumber++
	}
}

func parseFrames(frames map[int]message) strings.Builder {
	// We now have all the frames
	// Let us get theses messages and reconstruct the payload
	frameNumbers := make([]int, len(frames))
	i := 0
	for number, _ := range frames {
		frameNumbers[i] = number
		i++
	}
	sort.Ints(frameNumbers)

	// Reconstruct the payload
	sbPayload := strings.Builder{}
	for _, Number := range frameNumbers {
		sbPayload.WriteString(frames[Number].payload)
	}

	return sbPayload
}

func handleRequestFrames(udpConn *net.UDPConn, port string) map[int]message {
	frames := make(map[int]message)
	err := udpConn.SetReadDeadline(time.Now().Add(8 * time.Second))
	firstTimeReceived := false
	checkError(err)
	for {
		buf := make([]byte, 1024)
		n, err := udpConn.Read(buf)
		if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
			err = udpConn.SetReadDeadline(time.Now().Add(8 * time.Second))
			if firstTimeReceived {
				// check if there's a sequence missing before breaking
				frameNumbers := make([]int, len(frames))
				i := 0
				for number, _ := range frames {
					frameNumbers[i] = number
					i++
				}
				sort.Ints(frameNumbers)
				length := len(frameNumbers)
				dif := frameNumbers[len(frameNumbers)-1] - frameNumbers[0]
				if length-1 == dif {
					sbPayload := parseFrames(frames)
					req, err := request.Parse(sbPayload.String())
					if err != nil {
						// we are obviously missing data!
						// cannot correctly parse the request
						continue
					}
					// grab the content length header
					// check if it matches the payload
					contentLength := req.Headers["Content-Length"]
					if contentLength == "" {
						break
					}
					// remove space
					split := strings.Split(contentLength, " ")

					intContentLength, err := strconv.Atoi(split[1])
					checkError(err)

					if len(req.Body)-1 == intContentLength {
						break
					}
					continue
				}
			}
			// just keep waiting for client to send stuff
			continue
		}
		err = udpConn.SetReadDeadline(time.Now().Add(8 * time.Second))
		if err != nil {
			if e, ok := err.(net.Error); !ok || !e.Timeout() {
				continue
			}
		}

		// CHECK FOR DATA
		m := parseMessage(buf, n)
		if m.packetType != DATA {
			// ignore it and continue waiting
			continue
		}

		// set first time received to true
		firstTimeReceived = true

		// check to see if the frame is already there first
		frames[int(m.sequenceNumber)] = m
		ipAddress, _ := connectionGetPortAndIP(udpConn)

		payloadBuffer := make([]byte, 50)
		payloadBufferWriter := bytes.NewBuffer(payloadBuffer)
		payloadBufferWriter.WriteString("0")
		intPort, _ := strconv.Atoi(port)
		ackPacket := createMessage(ACK, int(m.sequenceNumber), ipAddress, intPort, payloadBufferWriter.Bytes())
		bAckPacket := convertMessageToBytes(ackPacket)

		log.Println("Data packet received frame #:" + strconv.Itoa(int(m.sequenceNumber)))

		// sending ACK
		_, err = udpConn.Write(bAckPacket.Bytes())
		log.Println("Ack packet sent for frame #:" + strconv.Itoa(int(m.sequenceNumber)))
		err = udpConn.SetReadDeadline(time.Now().Add(8 * time.Second))
	}
	return frames
}

func connectionGetPortAndIP(udpConn *net.UDPConn) (string, int) {
	split := strings.Split(udpConn.RemoteAddr().String(), ":")
	ipAddress := split[0]

	port, err := strconv.Atoi(split[1])
	checkError(err)
	return ipAddress, port
}

func split(buf []byte, lim int) [][]byte {
	var chunk []byte
	chunks := make([][]byte, 0, len(buf)/lim+1)
	for len(buf) >= lim {
		chunk, buf = buf[:lim], buf[lim:]
		chunks = append(chunks, chunk)
	}
	if len(buf) > 0 {
		chunks = append(chunks, buf[:len(buf)])
	}
	return chunks
}

func initiateHandshake(buf []byte, addr net.UDPAddr, n int) (*net.UDPConn, error, bytes.Buffer, bool, string) {
	log.Println("UDP client : ", addr)

	log.Println(buf[:n])
	m := parseMessage(buf, n)

	port := m.peerPort

	// check if SYN
	if m.packetType != SYN {
		// DROP THE PACKET and wait for a correct one.
		return nil, nil, bytes.Buffer{}, true, ""
	}

	// CREATE A NEW SOCKET
	udpConn, err := net.DialUDP("udp", nil, &addr)

	log.Println("Created Socket to Handle: " + udpConn.LocalAddr().String())
	log.Println("Handling Client connection from " + udpConn.RemoteAddr().String())

	// Send SYN/ACK
	payloadBuffer := make([]byte, 50)
	payloadBufferWriter := bytes.NewBuffer(payloadBuffer)
	payloadBufferWriter.WriteString("0")

	synAckMessage := createMessage(SYNACK, 2, m.peerAddress, int(m.peerPort), payloadBufferWriter.Bytes())
	bSynAckMessage := convertMessageToBytes(synAckMessage)

	log.Println("Sending Client a " + addr.IP.String() + ":" + strconv.Itoa(addr.Port) + " a SYN/ACK Packet")

	// start timer and read the message
	_, err = udpConn.Write(bSynAckMessage.Bytes())
	err = udpConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	checkError(err)
	count := 5
	for {
		buf := make([]byte, 1024)
		n, err := udpConn.Read(buf)
		if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
			// resend the SYN ACK Request
			_, err = udpConn.Write(bSynAckMessage.Bytes())
			log.Println("Attempting to resend SYN/ACK to " + addr.IP.String() + ":" + strconv.Itoa(addr.Port))
			count--
			err = udpConn.SetReadDeadline(time.Now().Add(5 * time.Second))
			if count < 0 {
				return nil, nil, bytes.Buffer{}, true, ""
			}
			continue
		}
		if err != nil {
			if e, ok := err.(net.Error); !ok || !e.Timeout() {
				continue
			}
		}

		// CHECK FOR ACK
		m := parseMessage(buf, n)

		log.Println("Received a " + strconv.Itoa(int(m.packetType)) + " packet as a response! from " + addr.String())

		if m.packetType == DATA {
			// We must have missed the last ACK, change state
			log.Println("Handshake Completed.")
			break
		}

		if m.packetType != ACK {
			// ignore it and continue waiting
			continue
		} else {
			log.Println("Handshake Completed.")
			break
		}
	}
	return udpConn, err, bSynAckMessage, false, strconv.Itoa(int(port))
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
	binary.BigEndian.PutUint16(portBuffer[:], uint16(port))

	sequenceNumberBuffer := [4]byte{}
	binary.BigEndian.PutUint32(sequenceNumberBuffer[:], uint32(sequenceNumber))

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

	port := binary.BigEndian.Uint16(buf[9:11])

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

func handleHTTP(opts *cli.Options, payload string) (responseString string) {
	req, err := request.Parse(payload)
	data, err := request.Handle(req, opts)
	if err != nil {
		httpError, _ := strconv.Atoi(err.Error())
		if req == nil {

		}
		return response.SendHTTPError(httpError, req.Protocol, req.Version)
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return response.SendHTTPError(http.StatusInternalServerError, req.Protocol, req.Version)
	}

	headers, _ := response.NewResponseHeaders(req, jsonData)
	return response.SendNewResponse(http.StatusOK, req.Protocol, req.Version, headers, string(jsonData))
}
