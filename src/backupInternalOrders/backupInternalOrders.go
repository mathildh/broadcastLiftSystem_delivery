package backupInternalOrders

import(
	"encoding/json"
	"io/ioutil"
	"fmt"
	"time"
	. "../orderController"
	. "../types"
)

func BackupInternalOrders_UpdateBackup(backupPath string){
	for{
		time.Sleep(2*time.Second)

		allOrders := OrderController_GetThisLiftsOrderQueue()
		internalOrders := make([]bool, 0)
		for _, floor := range allOrders{
			internalOrders = append(internalOrders, floor[ButtonType_INTERNAL])
		}
		byteMessage, _ := json.Marshal(internalOrders)
		if writeError := ioutil.WriteFile(backupPath, byteMessage, 0644); writeError!= nil{
			fmt.Println(writeError)
		}
		//backupFile.Write(byteMessage)
	}
}