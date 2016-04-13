package main

import (
	. "./liftDriver"
	. "./types"
	. "./network"
	. "./orderController"
	. "./backupInternalOrders"
	"fmt"
	"net"
	"time"
	"encoding/json"
	"os/exec"
	"os"
    "os/signal"
    "syscall"
)



func main() {
	ifPowerloss := false
	
	c := make(chan os.Signal, 1)
    signal.Notify(c, syscall.SIGINT)
    go func() {
        <-c
        fmt.Println("software crash procedure")
        os.Remove("backupInternalOrders/backup")
        LiftDriver_SetMotorDirection(MotorDirection_STOP)
    	os.Exit(1)    
    }()

    defer func(){
    	LiftDriver_SetMotorDirection(MotorDirection_STOP)
    	if ifPowerloss{
    		exec.Command("pkill", "main").Run()
    	}else{
    		fmt.Println("software crash")
    		os.Remove("backupInternalOrders/backup")
    		os.Exit(1)
    	}
    }()

    processPairPort := "6666"
	listenAddress, _ := net.ResolveUDPAddr("udp", "localhost:" + processPairPort)
	listenConnection, _ := net.ListenUDP("udp", listenAddress)
	data := make([]byte, 1024)
	existingPrimal := true

	var copyOrderQueue [TOTAL_FLOORS][TOTAL_BUTTON_TYPES]bool

	for existingPrimal{
		readingConnectionChannel := make(chan [TOTAL_FLOORS][TOTAL_BUTTON_TYPES]bool)
		timeoutChannel := make(chan bool)
		go func(){
			time.Sleep(1000*time.Millisecond)
			timeoutChannel <- true
		}()
		go func(){
			numberOfBytes, _, readingError := listenConnection.ReadFromUDP(data)
			if readingError == nil{
				var message [TOTAL_FLOORS][TOTAL_BUTTON_TYPES]bool
				jsonError := json.Unmarshal(data[:numberOfBytes], &message)
				if jsonError == nil{
					readingConnectionChannel <- message
				}
			}else{
				fmt.Println("Reading from UDP error", readingError)
			}
		}()
		select{
		case <- timeoutChannel:
			existingPrimal = false
			fmt.Println("Timeout!!")
			listenConnection.Close()
		case orderQueue :=<- readingConnectionChannel:
			copyOrderQueue = orderQueue
			fmt.Println("Primal exists: ", copyOrderQueue)
		}

	}


	cmd := exec.Command("gnome-terminal", "-e", "./main")
	cmd.Output()

	//BLITT PRIMAL!!!!
	go Network_SendPrimalMessage(processPairPort)
	LiftDriver_Initialize()

	var backupFile *os.File
	backupPath := "backupInternalOrders/backup"
	var errorBackupFile error
	if _, existError := os.Stat("backupInternalOrders/backup"); os.IsNotExist(existError) {
		//Backup eksisterer ikke = first time or softwarecrash
		fmt.Println("oppretter backup")

		//Oppretter backupfil
		backupFile, errorBackupFile = os.Create("backupInternalOrders/backup")
		//Sjekk om var child, om copyOrderQueue var fylt og oppdaterer deretter
		if errorBackupFile == nil{
			for floor:= 0; floor < TOTAL_FLOORS; floor++{	
				for buttonType := ButtonType_UP; buttonType <= ButtonType_INTERNAL; buttonType++ {
					if (copyOrderQueue[floor][buttonType] == true){
						buttonOrder := Button{Type: buttonType, Floor: floor}
						fmt.Println("Reading child's saved orders")
						OrderController_UpdateThisLiftsOrderQueue(buttonOrder,true)
						lamp := Lamp{ButtonOrder: buttonOrder, IfOn: true}
						LiftDriver_SetButtonLamp(lamp)
					}
				}
			}		
		}
	}else{
		//= powerloss
		fmt.Println("Henter backup")
		//Henter gamle interne ordre, oppdaterer orderQueue:
		backupFile, errorBackupFile = os.Open("backupInternalOrders/backup")
		defer backupFile.Close()
		if errorBackupFile == nil{
			bytesOfFile := make([]byte, 1024)
	    	numberOfBytes, _:= backupFile.Read(bytesOfFile)

	    	var backupInternalOrders [TOTAL_FLOORS]bool
			jsonError := json.Unmarshal(bytesOfFile[:numberOfBytes], &backupInternalOrders)
			fmt.Println("Fetched backupInternalOrders: ", backupInternalOrders)
			if jsonError == nil{
				for floor:=0; floor<TOTAL_FLOORS; floor++{
					if backupInternalOrders[floor] == true{
						buttonOrder := Button{Type: ButtonType_INTERNAL, Floor: floor}
						OrderController_UpdateThisLiftsOrderQueue(buttonOrder,true)
						lamp := Lamp{ButtonOrder: buttonOrder, IfOn: true}
						LiftDriver_SetButtonLamp(lamp)
					}
				}
			}
		}
	}

	detectIdleChannel := make(chan bool, 1)
	receiveMessageChannel := make(chan NetworkMessage)
	sendMessageChannel := make(chan NetworkMessage)
	setLampChannel := make(chan Lamp)
	floorChannel := make(chan int,1)
	deadLiftIPChannel := make(chan string)
	doorOpenChannel := make(chan bool, 1)
	doorCloseChannel := make(chan bool, 1)
	delegateOrderChannel := make(chan Button)

	broadcastPort := "30002"
	messageSize := 1024

	powerlossTimer := time.NewTimer(4*time.Second)
	powerlossTimer.Stop()

	ifNetworkInitSuccess := Network_Initialize(broadcastPort, messageSize, sendMessageChannel, receiveMessageChannel)
	if !ifNetworkInitSuccess{
		fmt.Println("Network initialization did not succeed.")
		os.Exit(1)
	}else{
		fmt.Println("Network initialization success")
	}

	go BackupInternalOrders_UpdateBackup(backupPath)
	go Network_HandleNetworkMessages(receiveMessageChannel, sendMessageChannel, setLampChannel)
	go Network_BroadcastStatus(sendMessageChannel)
	go LiftDriver_DetectButtonEvent(delegateOrderChannel)
	go LiftDriver_DetectFloorEvent(floorChannel)
	go OrderController_DelegateOrders(delegateOrderChannel, sendMessageChannel, setLampChannel)
	go OrderController_DetectDeadLifts(deadLiftIPChannel)
	go OrderController_TakeDeadLiftOrders(deadLiftIPChannel)

	detectIdleChannel <- true

	for (!ifPowerloss){
		select {
		case <- detectIdleChannel:
			powerlossTimer.Stop()
			currentFloor := LiftDriver_GetLastFloorOfLift()
			nextDirection := OrderController_GetNextDirection(MotorDirection_STOP, currentFloor, OrderController_GetThisLiftsOrderQueue())
			if nextDirection != MotorDirection_STOP{
				LiftDriver_SetMotorDirection(nextDirection)
				powerlossTimer.Reset(4*time.Second)
				fmt.Println("RESET timer in idle")
			}else{
				if OrderController_IfLiftShouldStop(currentFloor, MotorDirection_STOP, OrderController_GetThisLiftsOrderQueue()){
					doorOpenChannel <- true
				}else{
					go func(){
				 		time.Sleep(time.Millisecond*100)
				 		detectIdleChannel <- true
				 	}()
				}
			}
		case arrivalFloor := <- floorChannel:
			powerlossTimer.Stop()
			fmt.Println("floorChannel emptied, arrived at floor")
			LiftDriver_SetFloorIndicator(arrivalFloor)
			direction := LiftDriver_GetLastSetDirection()
			if OrderController_IfLiftShouldStop(arrivalFloor, direction, OrderController_GetThisLiftsOrderQueue()){
				LiftDriver_SetMotorDirection(MotorDirection_STOP)
				doorOpenChannel <- true
			}else{
				powerlossTimer.Reset(4*time.Second)
				fmt.Println("RESET timer after arrival")
			}
		case  <- doorOpenChannel:
			fmt.Println("Opening door...")
			LiftDriver_SetDoorLamp(1)
			currentFloor := LiftDriver_GetLastFloorOfLift()
			for buttonType := ButtonType_UP; buttonType <= ButtonType_INTERNAL; buttonType++ {
		 		button := Button{Type: buttonType, Floor: currentFloor}
		 		OrderController_UpdateThisLiftsOrderQueue(button, false)

		 		finishedOrder := LiftOrder{ButtonOrder: Button{Type: buttonType, Floor: currentFloor}}
				message := NetworkMessage{MessageType: NetworkMessageType_FinishedOrder, Order: finishedOrder}
				sendMessageChannel <- message
		 	}
		 	go func(){
		 		time.Sleep(time.Second*3)
		 		doorCloseChannel <- true
		 	}()
		case <-doorCloseChannel:
			currentFloor := LiftDriver_GetLastFloorOfLift()
			nextDirection := OrderController_GetNextDirection(MotorDirection_STOP, currentFloor, OrderController_GetThisLiftsOrderQueue())
			if nextDirection == MotorDirection_STOP{
				if OrderController_IfLiftShouldStop(currentFloor, MotorDirection_STOP, OrderController_GetThisLiftsOrderQueue()){
					doorOpenChannel <- true
				}else{
					LiftDriver_SetDoorLamp(0)
					detectIdleChannel <- true
				}
			}else{
				powerlossTimer.Reset(4*time.Second)
				fmt.Println("RESET timer after Door close")
				LiftDriver_SetDoorLamp(0)
				LiftDriver_SetMotorDirection(nextDirection)
			}
		case  lamp := <- setLampChannel:
			LiftDriver_SetButtonLamp(lamp)
		case <- powerlossTimer.C:
			fmt.Println("Powerloss!!")
			ifPowerloss = true
			break
		}
	}
}


