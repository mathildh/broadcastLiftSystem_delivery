package liftDriver

/*
#cgo CFLAGS: -std=gnu11
#cgo LDFLAGS: -lcomedi -lm
#include "io.h"
#include "elev.h"
*/
import "C"

import(
	"time"
	"fmt"
	. "../typesAndConstants"
)

/*
Variables needed in implementation of the two basic lift algorithms:
If the lift should stop at a current floor and in the calculation of the next motor direction thereafter.
*/
var lastSetDirection MotorDirection
var lastFloorOfLift int

func LiftDriver_GetLastFloorOfLift() int{
	return lastFloorOfLift
}

func LiftDriver_GetLastSetDirection() MotorDirection{
	return lastSetDirection
}

func LiftDriver_Initialize() {
	fmt.Println("Initializing liftDriver")
	C.elev_init( C.elev_type(C.ET_Comedi))
	lastFloorOfLift = LiftDriver_GetFloor()
	lastSetDirection = MotorDirection_STOP
}

func LiftDriver_SetMotorDirection(direction MotorDirection) {
	C.elev_set_motor_direction(C.elev_motor_direction_t(C.int(direction)))
	lastSetDirection = direction
}

func LiftDriver_SetButtonLamp(lamp Lamp) {
	button := lamp.ButtonOrder.Type
	floor := lamp.ButtonOrder.Floor
	ifOn := 1
	if lamp.IfOn == true{
		ifOn = 1
	}else{
		ifOn = 0
	}
	if !(floor == TOTAL_FLOORS-1 && button == ButtonType_UP) && !(floor == 0 && button == ButtonType_DOWN){
		C.elev_set_button_lamp(C.elev_button_type_t(C.int(button)), C.int(floor), C.int(ifOn))
	}
}

func LiftDriver_SetFloorIndicator(floor int) {
	C.elev_set_floor_indicator(C.int(floor))
}

func LiftDriver_SetDoorLamp(onOrOff int) {
	C.elev_set_door_open_lamp(C.int(onOrOff))
}

func LiftDriver_GetButtonSignal(button ButtonType, floor int) bool {
	return int(C.elev_get_button_signal(C.elev_button_type_t(C.int(button)), C.int(floor))) != 0
}

func LiftDriver_GetFloor() int {
	return int(C.elev_get_floor_sensor_signal())
}

func LiftDriver_DetectButtonPress(delegateOrderChannel chan Button) {
	var previousButtonValues [TOTAL_FLOORS][TOTAL_BUTTON_TYPES]bool
	for {
		time.Sleep(BUTTON_RATE)

		for floor := 0; floor < TOTAL_FLOORS; floor++ {
			for buttonType := ButtonType_UP; buttonType <= ButtonType_INTERNAL; buttonType++ {
				var ifButtonPressed bool = LiftDriver_GetButtonSignal(buttonType, floor)
				if ifButtonPressed && !previousButtonValues[floor][buttonType] {
					buttonOrder := Button{Type: buttonType, Floor: floor}
					delegateOrderChannel <- buttonOrder
					previousButtonValues[floor][buttonType] = true
				}else if ifButtonPressed != previousButtonValues[floor][buttonType] {
					previousButtonValues[floor][buttonType] = false
				}
			}
		}
	}
}

func LiftDriver_DetectNewFloor(arrivalFloorChannel chan int) {
	var previousFloor int = -1
	for {
		time.Sleep(FLOOR_RATE)
		var currentFloor int = LiftDriver_GetFloor()
		if currentFloor != previousFloor && currentFloor != -1 {
			lastFloorOfLift = currentFloor
			arrivalFloorChannel <- currentFloor
			previousFloor = currentFloor
		}
	}
}
