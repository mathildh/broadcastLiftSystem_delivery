package network_ver2

import (
	"encoding/json"
	"net"
	"fmt"
	"time"
	"strings"
	"os"
	. "../types"
	. "../liftDriver"
	. "../orderController"
	)

var broadcastAddress *net.UDPAddr
var liftIP string

func Network_GetLocalIP() string {
	return liftIP
}

func Network_Initialize(broadcastPort string, messageSize int, sendMessageChannel, receiveMessageChannel chan NetworkMessage) bool {
	callerAddress := &net.UDPAddr{
		Port: 20011,
		IP:   net.IPv4bcast,
	}
	tempConnection, _ := net.DialUDP("udp4", nil, callerAddress)
	tempAddress := tempConnection.LocalAddr()
	liftIP = strings.Split(tempAddress.String(), ":")[0] // only want ip
	tempConnection.Close()

	address, resolvingError := net.ResolveUDPAddr("udp4", "255.255.255.255:" + broadcastPort)
	broadcastAddress = address
	if resolvingError != nil {
		return false
	}

	broadcastConnection, listenError := net.ListenUDP("udp", broadcastAddress)
	if listenError != nil {
		return false
	}
	go Network_UDPReceiveMessage(broadcastConnection, messageSize, receiveMessageChannel, sendMessageChannel)
	go Network_UDPSendMessage(broadcastConnection, sendMessageChannel)

	return true
}

func Network_UDPSendMessage(broadcastConnection *net.UDPConn, sendMessageChannel chan NetworkMessage) {
	for {
		message := <-sendMessageChannel
		fmt.Println("Sending message: ")
		fmt.Println("Type: ", message.MessageType)
		if message.MessageType != NetworkMessageType_LiftStatus{
			fmt.Println(message)
		}
		byteMessage, marshalError := json.Marshal(message)
		Network_CheckIfError(marshalError, "Error marshaling in Network_UDPSendMessage")
		_, sendingError := broadcastConnection.WriteToUDP(byteMessage, broadcastAddress)
		
		Network_CheckIfError(sendingError, "Error sending in Network_UDPSendMessage")
	}
}

func Network_UDPReceiveMessage(broadcastConnection *net.UDPConn, messageSize int, receiveMessageChannel chan NetworkMessage, sendMessageChannel chan NetworkMessage) {
	data := make([]byte, messageSize)
	for {

		numberBytes, _, readingError := broadcastConnection.ReadFromUDP(data)
		Network_CheckIfError(readingError, "Error when reading in Network_UDPReceiveMessage")
		var message NetworkMessage
		jsonError := json.Unmarshal(data[:numberBytes], &message)
		Network_CheckIfError(jsonError, "Error when Unmarshalling in Network_UDPReceiveMessage")
		
		receiveMessageChannel <- message
	}
}

func Network_BroadcastStatus(sendMessageChannel chan NetworkMessage){
	for{
		time.Sleep(time.Millisecond*200)
		
		liftStatus := LiftStatus{LiftIP: liftIP, Direction: LiftDriver_GetLastSetDirection(), LastFloor: LiftDriver_GetLastFloorOfLift()}
		liftStatus.Timestamp = time.Now()
		liftStatus.OrderQueue =  OrderController_GetThisLiftsOrderQueue()
		message := NetworkMessage{MessageType: NetworkMessageType_LiftStatus, Status: liftStatus} 
		
		sendMessageChannel <-message
	}
}

func Network_HandleNetworkMessages(receiveMessageChannel chan NetworkMessage, sendMessageChannel chan NetworkMessage, setLampChannel chan Lamp){
	for{
		message := <- receiveMessageChannel

		fmt.Println("Message being handled...", message.MessageType)
		switch message.MessageType{
		case NetworkMessageType_NewOrder:
			newLiftOrder := message.Order
			if newLiftOrder.LiftIP == liftIP{
				OrderController_UpdateThisLiftsOrderQueue(newLiftOrder.ButtonOrder, true)
			}
			lampToBeSet := Lamp{ButtonOrder:newLiftOrder.ButtonOrder, IfOn: true}
			setLampChannel <- lampToBeSet
		
		case NetworkMessageType_LiftStatus:
			liftStatus := message.Status
			OrderController_UpdateLiftStatus(liftStatus)
		case NetworkMessageType_FinishedOrder:
			finishedLiftOrder := message.Order
			lampToBeSet := Lamp{ButtonOrder:finishedLiftOrder.ButtonOrder, IfOn: false}
			setLampChannel <- lampToBeSet
		}
	}
}

func Network_SendPrimalMessage(port string){
	receiverAddress, _ := net.ResolveUDPAddr("udp", ":" + port)
	sendPrimalConnection, _ := net.DialUDP("udp", nil, receiverAddress)
	defer sendPrimalConnection.Close()

	for {
		orderQueue := OrderController_GetThisLiftsOrderQueue()
		byteMessage, _ := json.Marshal(orderQueue)
		sendPrimalConnection.Write(byteMessage)
		//fmt.Fprintf(sendPrimalConnection, "I'm primal!!!")
		time.Sleep(500*time.Millisecond)
	}
}

func Network_CheckIfError(runningError error, errorMessage string){
	if runningError != nil{
		fmt.Println("Error of type: " + errorMessage)
		fmt.Println(runningError)
		os.Exit(1)
	}
}
