# Broadcast Liftsystem
 This is a project for running multiple lifts on the same broadcast network, including fault tolerance and an approach to optimal service. A lift in the system will detect that a button is pressed, calculate the optimal lift for handling the order and broadcast this result to all lifts in the system. The lift specified in the broadcast message will detect the order and handle it. Furthermore, all lifts broadcast a status message that includes a copy of their order queue which the calculations of the optimal lift are based on, and messages that enables synchronization of light settings.
 
# Functionality
 This lift system:

1. Includes any number floors and any number of lifts in the broadcast network, specified by a port number.
2. Ensures that no internal orders are lost and no external orders are lost when there are at least one lift in the network.
 
# Design
Robustness of system:
- Handles software crash and powerloss by going to fail-safe mode. 
- In case of software crash: The child process continously receives a copy of the lift's order queue and will take over and continue to operate the lift.
- In case of powerloss: Backup of the internal orders in the system has been written to file and will be read during the next initialization.

Module responsibilities:
- Module liftDriver: Includes functions that reads and writes to hardware. Furthermore, it detects button presses and arrival of lift at a new floor.
- Module network: Uses UDP. Call to initalization function will make the system receive and send broadcast messages continously. Received messages are handled by at message handler-function. In addition, one function will periodically broadcasts the lifts status and another will periodically send a copy of the lift's order queue to a child process using localhost. 
- Module orderController: Includes functions that updates the order queue of the lift, checks if the lift should stop at the current floor and calculates the next direction of the lift. The module has a function that delegates new orders using a costfunction. Furthermore, it keeps track of statuses of the other lifts in the network in order to run the costfunction, and handles orders of dead lifts.
- Module typesAndConstants: Defines number of floors and buttons in the system, the periods of some threads and custom made types.

Network protocol:     
- Defined by the custom made types.
- The type of network message is specified by a type field in the network message struct and used to read the correct fields in the network message.
- The network message struct is sent over UDP by using the included package json.

 Weaknesses:
 - Cannot take new orders during the con child process takes over.
 - Ordercontroller keeps track of the statuses of all lifts and detects that a lift has left the network. This functionality could be separated into a new module but this is a tradeoff between the size of the system, with a growing number of modules, and less module responsibility. 
