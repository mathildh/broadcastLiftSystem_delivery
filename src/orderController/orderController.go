package orderController

import(
	. "../types"
	"time"
	"fmt"
)

var thisLiftsOrderQueue [TOTAL_FLOORS][TOTAL_BUTTON_TYPES]bool //init right

var previousStatusOfAllLifts = make(map[string]LiftStatus)

var activeLifts = make(map[string]bool)


func OrderController_GetThisLiftsOrderQueue() [TOTAL_FLOORS][TOTAL_BUTTON_TYPES]bool {
	return thisLiftsOrderQueue
}

func OrderController_UpdateThisLiftsOrderQueue(order Button, ifActive bool) {
	thisLiftsOrderQueue[order.Floor][order.Type] = ifActive
}

func OrderController_UpdateLiftStatus(newLiftStatus LiftStatus) {
	previousStatusOfAllLifts[newLiftStatus.LiftIP] = newLiftStatus
	activeLifts[newLiftStatus.LiftIP] = true
}


func OrderController_DelegateOrders(delegateOrderChannel chan Button, sendMessageChannel chan NetworkMessage, setLampChannel chan Lamp){
	
	for{

		buttonOrder := <- delegateOrderChannel
		if buttonOrder.Type == ButtonType_INTERNAL{
			lamp := Lamp{ButtonOrder: buttonOrder, IfOn: true}
			setLampChannel <- lamp
			OrderController_UpdateThisLiftsOrderQueue(buttonOrder, true)		
		}else{
			liftIP := OrderController_CostFunction(buttonOrder)
			fmt.Println("Lift taking the order: ", liftIP) 

			if liftIP == "noNetwork"{
					OrderController_UpdateThisLiftsOrderQueue(buttonOrder, true)
					setLampChannel <- Lamp{ButtonOrder: buttonOrder, IfOn: true}
			}else{
				liftOrder := LiftOrder{LiftIP: liftIP, ButtonOrder: buttonOrder}
				message := NetworkMessage{MessageType: NetworkMessageType_NewOrder, Order: liftOrder}
				
				sendMessageChannel <- message
			}
		}
	}
}

//see pseudo code TTK4145/Prosject/algo
func OrderController_GetNextDirection(previousDirection MotorDirection, currentFloor int, orderQueue [TOTAL_FLOORS][TOTAL_BUTTON_TYPES]bool) MotorDirection{
	ifOrdersAbove := false
	ifOrdersBelow := false

	for floor:= currentFloor; floor < TOTAL_FLOORS; floor++{	
		for buttonType := ButtonType_UP; buttonType <= ButtonType_INTERNAL; buttonType++ {
			if (orderQueue[floor][buttonType] == true){
				if floor == currentFloor{
					return MotorDirection_STOP
				}else{
					ifOrdersAbove = true
					break
				}
			}
		}
	}
	for floor:= currentFloor; floor >= 0; floor--{	
		for buttonType := ButtonType_UP; buttonType <= ButtonType_INTERNAL; buttonType++ {
			if (orderQueue[floor][buttonType] == true){
				ifOrdersBelow = true
				break
			}
		}
	}
	if ifOrdersAbove && (previousDirection == MotorDirection_UP){
		return MotorDirection_UP
	}else if ifOrdersBelow && (previousDirection == MotorDirection_DOWN){
		return MotorDirection_DOWN
	}else if ifOrdersAbove{
		return MotorDirection_UP
	}else if ifOrdersBelow{
		return MotorDirection_DOWN
	}else{
		return MotorDirection_STOP
	}
}

func OrderController_IfLiftShouldStop(currentFloor int, currentDirection MotorDirection, orderQueue [TOTAL_FLOORS][TOTAL_BUTTON_TYPES]bool) bool{
	//Bestilt i riktig retning

	var queueIndex int
	if currentDirection == MotorDirection_UP{
		queueIndex = 0
	}else if currentDirection == MotorDirection_DOWN{
		queueIndex = 1
	}else {
		for buttonType := ButtonType_UP; buttonType <= ButtonType_INTERNAL; buttonType++ {
	 		if orderQueue[currentFloor][buttonType] == true{
	 			return true
	 		}
	 	}
	 	return false
	}
	//Bestilt i riktig retning
	if orderQueue[currentFloor][queueIndex] == true{
		return true
	}
	//Bestilt intern
	if orderQueue[currentFloor][2] == true{
		return true
	}
	//Bestilt i etasjene i nåværende retning over/under currentfloor
	for floor:=currentFloor+int(currentDirection); (floor < TOTAL_FLOORS && floor >= 0); floor=floor+int(currentDirection){
		for buttonType := ButtonType_UP; buttonType <= ButtonType_INTERNAL; buttonType++ {
	 		if orderQueue[floor][buttonType] == true{
	 			return false
	 		}
	 	}
	}
	return true
}


func OrderController_CostFunction(buttonOrder Button) string {
	minimumDuration := int(time.Second/1e9)*1000000
	optimalLiftIP := "noNetwork"

	travelTime := 3*int(time.Second/1e9)
	doorOpenTime := 3*int(time.Second/1e9)

	for liftIP, liftStatus := range previousStatusOfAllLifts{
		//Hvis har tilgang til nettverk
		if activeLifts[liftIP] == true{
			duration := 0
			orderQueue := liftStatus.OrderQueue
			orderQueue[buttonOrder.Floor][buttonOrder.Type] = true
			
			if liftStatus.Direction != MotorDirection_STOP{
				duration += travelTime/2
			}
			for{
				//Har nådd etasje, sjekker om stoppe, ellers kjøre til neste etasje
				arrivalFloor := liftStatus.LastFloor + int(liftStatus.Direction)
				liftStatus.LastFloor = arrivalFloor
				shouldStop := OrderController_IfLiftShouldStop(arrivalFloor, liftStatus.Direction, orderQueue)
				if shouldStop{
					if arrivalFloor == buttonOrder.Floor{
						break
					}
					previousDirection := liftStatus.Direction
					liftStatus.Direction = MotorDirection_STOP
					for buttonType := ButtonType_UP; buttonType <= ButtonType_INTERNAL; buttonType++ {
				 		orderQueue[arrivalFloor][buttonType] = false
				 	}
				 	duration += doorOpenTime
				 	liftStatus.Direction = OrderController_GetNextDirection(previousDirection, arrivalFloor, orderQueue)
				 	duration += travelTime
				}else{
					//Kjører til neste etasje
					if liftStatus.Direction == MotorDirection_STOP{
						liftStatus.Direction = OrderController_GetNextDirection(liftStatus.Direction, arrivalFloor, orderQueue)
					}
					duration += travelTime
				}
			}
			if duration < minimumDuration{
				minimumDuration = duration
				optimalLiftIP = liftIP
			}
		}
	}
	return optimalLiftIP
}

func OrderController_DetectDeadLifts(deadLiftIPChannel chan string){
	for{
		time.Sleep(time.Millisecond*100)
		for liftIP, liftStatus := range previousStatusOfAllLifts{
			if activeLifts[liftIP] && (time.Now().Sub(liftStatus.Timestamp) > 3*time.Second){
				activeLifts[liftIP] = false
				deadLiftIPChannel <- liftIP
				fmt.Println("Dead lift detected: ", liftIP)
			}
		}
	}
}

func OrderController_TakeDeadLiftOrders(deadLiftIPChannel chan string){
	for{
		deadLiftIP := <- deadLiftIPChannel
		deadLiftOrders := previousStatusOfAllLifts[deadLiftIP].OrderQueue
		fmt.Println("Dead lift orders is taken: ", deadLiftOrders)

		fmt.Println("Active lifts: ", activeLifts)
		for floor := 0; floor < TOTAL_FLOORS; floor++ {
			fmt.Println("Floor: ", floor)
			for buttonType := ButtonType_UP; buttonType < ButtonType_INTERNAL; buttonType++ {
				fmt.Println("buttonType: ", buttonType)	
				fmt.Println("result: ", deadLiftOrders[floor][buttonType])
				if deadLiftOrders[floor][buttonType] == true{
					buttonOrder := Button{Type: buttonType, Floor: floor}
					fmt.Println("Taking dead lift order: ", buttonOrder)
					OrderController_UpdateThisLiftsOrderQueue(buttonOrder,true)
				}
			}
		}
	}
}