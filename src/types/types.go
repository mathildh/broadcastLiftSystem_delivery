package types

import (
	"time"
)

const TOTAL_FLOORS = 4
const TOTAL_BUTTON_TYPES = 3

type MotorDirection int
type ButtonType int

const (
	MotorDirection_DOWN MotorDirection = iota - 1
	MotorDirection_STOP
	MotorDirection_UP
)

const (
	ButtonType_UP ButtonType = iota
	ButtonType_DOWN
	ButtonType_INTERNAL
)

type NetworkMessageType int

const (
	NetworkMessageType_LiftStatus = iota
	NetworkMessageType_NewOrder   //ops, vi sender egentlig noe to ganger til samme heis...?
	NetworkMessageType_FinishedOrder //sende over buttonOrder
)

type NetworkMessage struct {
	MessageType NetworkMessageType
	Status      LiftStatus
	Order       LiftOrder

}

type LiftStatus struct {
	LiftIP     string
	Direction  MotorDirection
	LastFloor  int
	Timestamp  time.Time
	OrderQueue [TOTAL_FLOORS][TOTAL_BUTTON_TYPES]bool
}

type Button struct {
	Type ButtonType
	Floor  int
}

type LiftOrder struct {
	LiftIP string
	ButtonOrder  Button
}

type Lamp struct{
	ButtonOrder Button
	IfOn bool
}

