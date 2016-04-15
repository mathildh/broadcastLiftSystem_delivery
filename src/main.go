package main

import (
	. "./liftDriver"
	. "./typesAndConstants"
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
	ifNetworkInInitialization := false
	/*
	Procedure catching ^C in order to simulate software crash.
	*/
	c := make(chan os.Signal, 1)
    signal.Notify(c, syscall.SIGINT)
    go func() {
        <-c
        os.Remove("backupInternalOrders/backup")
        LiftDriver_SetMotorDirection(MotorDirection_STOP)
    	os.Exit(1)    
    }()

    /*
	Implementing process pairs: 
	Resolving if process is primal or child. 
	Child receives copy of the lifts order queue from primal.

    Defer func:
    Procedure ensuring a fail-safe mode of the system.
    Backup of internal orders are  saved in case of powerloss, and the whole program (child and primal) is killed. 
    This is to ensure that the dead lift will not be taken into account by the other lifts. 
    The whole program is also killed when there is no network connection during initialization. 
    Else, the take over by the child process ensures completion of orders in the system.
    */
    defer func(){
    	LiftDriver_SetMotorDirection(MotorDirection_STOP)
    	if ifPowerloss || !ifNetworkInInitialization{
    		time.Sleep(time.Second)
    		exec.Command("pkill", "main").Run()
    	}else{
    		fmt.Println("Software crash")
    		os.Remove("backupInternalOrders/backup")
    		os.Exit(1)
    	}
    }()

    existingPrimal := true
    processPairPort := "6656"
	listenAddress, _ := net.ResolveUDPAddr("udp", "localhost:" + processPairPort)
	listenConnection, _ := net.ListenUDP("udp", listenAddress)

	var copyOrderQueue [TOTAL_FLOORS][TOTAL_BUTTON_TYPES]bool

	data := make([]byte, 1024)
	for existingPrimal{
		readingConnectionChannel := make(chan [TOTAL_FLOORS][TOTAL_BUTTON_TYPES]bool)
		timeoutChannel := make(chan bool)
		go func(){
			time.Sleep(EXISTING_PRIMAL_LIMIT)
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
			listenConnection.Close()
		case orderQueue :=<- readingConnectionChannel:
			copyOrderQueue = orderQueue
			fmt.Println("Child running")
		}
	}

	fmt.Println("Becoming Primal")
	go Network_SendPrimalMessage(processPairPort)
	LiftDriver_Initialize()

	cmd := exec.Command("gnome-terminal", "-e", "./main")
	cmd.Output()

	/*
	A backup file only exists if the lift experienced a power loss or no network connection in the initialization. 
	Hence, there will be no orders in the copy of the order queue from the process pair solution and only internal 
	orders from backup file will be completed.
	*/
	var backupFile *os.File
	backupPath := "backupInternalOrders/backup"
	var errorBackupFile error

	if _, existError := os.Stat("backupInternalOrders/backup"); os.IsNotExist(existError) {
		backupFile, errorBackupFile = os.Create("backupInternalOrders/backup")
		if errorBackupFile == nil{
			for floor:= 0; floor < TOTAL_FLOORS; floor++{	
				for buttonType := ButtonType_UP; buttonType <= ButtonType_INTERNAL; buttonType++ {
					if (copyOrderQueue[floor][buttonType] == true){
						buttonOrder := Button{Type: buttonType, Floor: floor}
						OrderController_UpdateThisLiftsOrderQueue(buttonOrder,true)
						lamp := Lamp{ButtonOrder: buttonOrder, IfOn: true}
						LiftDriver_SetButtonLamp(lamp)
					}
				}
			}		
		}
	}else{
		backupFile, errorBackupFile = os.Open("backupInternalOrders/backup")
		defer backupFile.Close()
		if errorBackupFile == nil{
			bytesFromBackupFile := make([]byte, 1024)
			var backupInternalOrders [TOTAL_FLOORS]bool
			numberOfBytes, _:= backupFile.Read(bytesFromBackupFile)
			jsonError := json.Unmarshal(bytesFromBackupFile[:numberOfBytes], &backupInternalOrders)
			if jsonError == nil{
				fmt.Println("Reading from backup")
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
	setLampChannel := make(chan Lamp,10)
	arrivalFloorChannel := make(chan int,1)
	deadLiftIPChannel := make(chan string)
	doorOpenChannel := make(chan bool, 1)
	doorCloseChannel := make(chan bool, 1)
	delegateOrderChannel := make(chan Button)

	broadcastPort := "30021"
	messageSize := 1024

	powerlossTimer := time.NewTimer(POWERLOSS_LIMIT)
	powerlossTimer.Stop()
	
	ifNetworkInInitialization = Network_Initialize(broadcastPort, messageSize, sendMessageChannel, receiveMessageChannel)
	fmt.Println("Network? ",ifNetworkInInitialization)
	
	/*
	Threads that detects events that should result in hardware changes are catched and handled 
	in the main thread.

	The threads handling dead lifts in the network, message sending and receiving, detection
	of button presses and delegating orders run in "the background" without interfering the main thread.

	A timer is set each time the lifts starts moving in order to detect powerloss.
	This will happen if the lift does not arrive at the next floor within a time limit.
	*/
	go BackupInternalOrders_UpdateBackup(backupPath)
	go Network_HandleNetworkMessages(receiveMessageChannel, sendMessageChannel, setLampChannel)
	go Network_BroadcastStatus(sendMessageChannel)
	go LiftDriver_DetectButtonPress(delegateOrderChannel)
	go LiftDriver_DetectNewFloor(arrivalFloorChannel)
	go OrderController_DelegateOrders(delegateOrderChannel, sendMessageChannel, setLampChannel)
	go OrderController_DetectDeadLifts(deadLiftIPChannel)
	go OrderController_TakeDeadLiftOrders(deadLiftIPChannel)

	detectIdleChannel <- true

	for (!ifPowerloss  && ifNetworkInInitialization){
		select {
		case <- detectIdleChannel:
			powerlossTimer.Stop()
			nextDirection := OrderController_GetNextDirection(MotorDirection_STOP, LiftDriver_GetLastFloorOfLift(), OrderController_GetThisLiftsOrderQueue())
			if nextDirection != MotorDirection_STOP{
				LiftDriver_SetMotorDirection(nextDirection)
				powerlossTimer.Reset(POWERLOSS_LIMIT)
			}else{
				if OrderController_IfLiftShouldStop(LiftDriver_GetLastFloorOfLift(), MotorDirection_STOP, OrderController_GetThisLiftsOrderQueue()){
					doorOpenChannel <- true
				}else{
					go func(){
				 		time.Sleep(IDLE_RATE)
				 		detectIdleChannel <- true
				 	}()
				}
			}
		case arrivalFloor := <- arrivalFloorChannel:
			powerlossTimer.Stop()
			LiftDriver_SetFloorIndicator(arrivalFloor)
			if OrderController_IfLiftShouldStop(arrivalFloor, LiftDriver_GetLastSetDirection(), OrderController_GetThisLiftsOrderQueue()){
				LiftDriver_SetMotorDirection(MotorDirection_STOP)
				doorOpenChannel <- true
			}else{
				powerlossTimer.Reset(POWERLOSS_LIMIT)
			}
		case  <- doorOpenChannel:
			LiftDriver_SetDoorLamp(1)
			currentFloor := LiftDriver_GetLastFloorOfLift()
			for buttonType := ButtonType_UP; buttonType <= ButtonType_INTERNAL; buttonType++ {
		 		button := Button{Type: buttonType, Floor: currentFloor}
		 		OrderController_UpdateThisLiftsOrderQueue(button, false)
		 		lamp := Lamp{ButtonOrder: button, IfOn: false}
		 		setLampChannel <- lamp
		 		finishedOrder := LiftOrder{ButtonOrder: Button{Type: buttonType, Floor: currentFloor}}
				message := NetworkMessage{MessageType: NetworkMessageType_FinishedOrder, Order: finishedOrder}
				sendMessageChannel <- message
		 	}
		 	go func(){
		 		time.Sleep(DOOR_OPEN_TIME)
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
				powerlossTimer.Reset(POWERLOSS_LIMIT)
				LiftDriver_SetDoorLamp(0)
				LiftDriver_SetMotorDirection(nextDirection)
			}
		case  lamp := <- setLampChannel:
			LiftDriver_SetButtonLamp(lamp)
		case <- powerlossTimer.C:
			fmt.Println("Powerloss!!")
			ifPowerloss = true
		}
	}
}


