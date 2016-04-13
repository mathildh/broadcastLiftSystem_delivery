# Broadcast Liftsystem
 This is a project for running multiple lifts on the same broadcast network, including fault tolerance and an approach to optimal service. A lift in the system will detect that a button is pressed, calculate the optimal lift for handling the order and broadcast this result to all lifts in the system. The lift specified in the broadcast message will detect the order and handle it. Furthermore, all lifts broadcast a status message that includes a copy of their order queue, which the calculations of the optimal lift are based on, and messages that enables synchronization of light settings.
 
# Functionality
 This lift network can:

1. Include any number of lifts in one broadcast network, specified by port number, and with any number of floors.
2. No internal orders are lost, assuming one lift is always on the network.
3. Different levels of fault tolerance, which ensures that no internal orders are lost.
 
# Design
Robustness of system:
- Handles software crash and powerloss by going to a fail-safe state 
- Fail-safe state: The lift is stopped and no internal orders are lost. Drives to valid floor when reinitialized
- In case of software crash: The child process continously receives a copy of the lifts order queue and will take over and continue to operate the lift.
- In case of powerloss: Backup of the internal orders in the system has been written to file and will be read during the next initialization.


Module responsibilities
- Module liftDriver: Includes functions that reads and writes to hardware. Furthermore, it detects button presses and arrival of lift at new floor.
- Module network: Uses UDP. Call to initalization function will make the system receive and send broadcast messages continously. Received messages are handled by at message handler-function. In addition, one function will periodically broadcasts the lifts status and another will periodically send a copy of the lifts order queue to a child process using localhost. 
- Module orderController: Incules functions that updates the order queue of the lift, checks if the lift should stop at the current floor and calculates the next direction of the lift. The module has a function that delegates new orders uses a costfunction. Furthermore, it keeps track of statuses of the other lifts in the network in order to run the costfunction. 

Network protocol:     

 Weaknesses:
 - Cannot take new orders during the con child process takes over.
 - Ordercontroller keeps track of the statuses of all lifts and detects that a lift has left the network. This functionality could be separated into a new module but this is a tradeoff between the size of the system, with a growing number of modules, and less module responsibility. 
