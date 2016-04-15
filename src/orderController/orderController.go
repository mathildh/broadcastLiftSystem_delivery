package orderController

import(
	. "../typesAndConstants"
	"time"
	"fmt"
)

var thisLiftsOrderQueue [TOTAL_FLOORS][TOTAL_BUTTON_TYPES]bool
/*
The variables below are used to calculate which lift in the network is the optimal for an external order
and to detect when to take over unhandled orders in the system due to death of a lift in network.
*/
var previousStatusOfAllLifts = make(map[string]LiftStatus)
var activeLifts = make(map[string]bool)

func OrderController_GetThisLiftsOrderQueue() [TOTAL_FLOORS][TOTAL_BUTTON_TYPES]bool {
	return thisLiftsOrderQueue
}

func OrderController_UpdateThisLiftsOrderQueue(order Button, ifActive bool) {
	thisLiftsOrderQueue[order.Floor][order.Type] = ifActive
}

/*
The function receives statuses of all lifts in the network and updates respectively.
*/
func OrderController_UpdateLiftStatus(newLiftStatus LiftStatus) {
	previousStatusOfAllLifts[newLiftStatus.LiftIP] = newLiftStatus
	activeLifts[newLiftStatus.LiftIP] = true
}

/*
If the lift is disconnected from the network, the cost function (calculateOptimalLiftIP)will 
return "noNetwork", and the order is delegated to the lift in the system.
*/
func OrderController_DelegateOrders(delegateOrderChannel chan Button, sendMessageChannel chan NetworkMessage, setLampChannel chan Lamp){
	for{
		buttonOrder := <- delegateOrderChannel
		if buttonOrder.Type == ButtonType_INTERNAL{
			lamp := Lamp{ButtonOrder: buttonOrder, IfOn: true}
			setLampChannel <- lamp
			OrderController_UpdateThisLiftsOrderQueue(buttonOrder, true)		
		}else{
			liftIP := OrderController_CalculateOptimalLiftIP(buttonOrder)
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

/*
Pseudocode for cost function:
For each of the lifts in the network:
1. Delegate the order in question to the lift.
2. Calculate an approximation of the time it will take for the lift to handle the order.
Return the IP of the lift with the lowest handling time.
*/
func OrderController_CalculateOptimalLiftIP(buttonOrder Button) string {
	minimumDuration := int(time.Second/1e9)*1000000
	optimalLiftIP := "noNetwork"

	travelTime := 3*int(time.Second/1e9)
	doorOpenTime := 3*int(time.Second/1e9)

	for liftIP, liftStatus := range previousStatusOfAllLifts{
		if activeLifts[liftIP] == true{
			duration := 0
			orderQueue := liftStatus.OrderQueue
			orderQueue[buttonOrder.Floor][buttonOrder.Type] = true
			
			if liftStatus.Direction != MotorDirection_STOP{
				duration += travelTime/2
			}
			for{
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

/*
The two following functions are based on preferring to continue in the direction of travel, as long
as there are any requests in that direction.
*/
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

/*
Checks
1. If there are orders in the current floor in the direction of the travel.
2. If there are internal orders.
3. If there are orders in the direction of the travel away from the current floor.
*/
func OrderController_IfLiftShouldStop(currentFloor int, currentDirection MotorDirection, orderQueue [TOTAL_FLOORS][TOTAL_BUTTON_TYPES]bool) bool{
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
	if orderQueue[currentFloor][queueIndex] == true{
		return true
	}
	if orderQueue[currentFloor][ButtonType_INTERNAL] == true{
		return true
	}
	for floor:=currentFloor+int(currentDirection); (floor < TOTAL_FLOORS && floor >= 0); floor=floor+int(currentDirection){
		for buttonType := ButtonType_UP; buttonType <= ButtonType_INTERNAL; buttonType++ {
	 		if orderQueue[floor][buttonType] == true{
	 			return false
	 		}
	 	}
	}
	return true
}

/*
A lift is defined dead, hence, not active, when the last received status is older than three seconds.
*/
func OrderController_DetectDeadLifts(deadLiftIPChannel chan string){
	for{
		time.Sleep(DETECT_DEAD_LIFT_RATE)
		for liftIP, liftStatus := range previousStatusOfAllLifts{
			if activeLifts[liftIP] && (time.Now().Sub(liftStatus.Timestamp) > DEAD_LIFT_LIMIT){
				activeLifts[liftIP] = false
				deadLiftIPChannel <- liftIP
				fmt.Println("Dead lift detected: ", liftIP)
			}
		}
	}
}

/*
The lift in the system will take all the orders of the dead lift itself. Hence, if there are 
more lifts in the system detecting the death of a lift, this solution results in redundant order
handling. 
*/
func OrderController_TakeDeadLiftOrders(deadLiftIPChannel chan string){
	for{
		deadLiftIP := <- deadLiftIPChannel
		deadLiftOrders := previousStatusOfAllLifts[deadLiftIP].OrderQueue
		fmt.Println("Taking dead lift's orders")
		for floor := 0; floor < TOTAL_FLOORS; floor++ {
			for buttonType := ButtonType_UP; buttonType < ButtonType_INTERNAL; buttonType++ {
				if deadLiftOrders[floor][buttonType] == true{
					buttonOrder := Button{Type: buttonType, Floor: floor}
					fmt.Println("Order taken: ", buttonOrder)
					OrderController_UpdateThisLiftsOrderQueue(buttonOrder,true)
				}
			}
		}
	}
}