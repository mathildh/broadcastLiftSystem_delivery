# Broadcast Liftsystem
 This is a project for running multiple lifts on the same broadcast network, including fault tolerance and an approach to optimal service. A lift in the system will detect that a button is pressed, calculate the optimal lift for handling the order and broadcast this result to all lifts in the system. The lift specified in the broadcast message will detect the order and handle it. Furthermore, all lifts broadcast a status message that includes a copy of their order queue which the calculations of the optimal lift are based on, and messages that enables synchronization of light settings.
 
# Functionality
 This lift system:

1. Includes any number floors and any number of lifts in the broadcast network, specified by a port number.
2. Ensures that no internal orders are lost and no external orders are lost when there are at least one lift in the network.
 
# Design
Definitions:
- Active lift: Lift that communicates over the broadcast network
- Order queue: The orders that needs to be handled saved as a twodimensional array. Rows corresponds to floors and columns to different button types.
- Cost function: The function in the system that calculates the optimal lift that should take a specific order.

Robustness of system:
- Handles software crash and powerloss by going to fail-safe mode. 
- In case of software crash: The Process Pairs-scheme is implemented. The child process continously receives a copy of the lift's order queue and will take over and continue to operate the lift.
- In case of powerloss or no network during initialization: Backup of the internal orders in the system has been written to file and will be read during the next initialization.

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
- The variables in the code that holds the orders a lift are responsible of have been named using the term order queue. These variables should rather have been named by using the term order array as these variables are two dimensional arrays with rows referring to floor and columns referring to different buttons, and not a queue in the sense of popping and pushing orders to the data structure. This revelation was unfortunately made very close to deadline, and we therefore decided to keep the slightly misleading names in the code. Ensuring transferability is also the cause of using the term order queue in this readme-file.
- The system does not detect button presses at a specific lift during a child process take-over at this lift.
- In case of a dead lift; all lifts remaining in the network will take the orders of this dead lift themselves. This solution result in redundant order handling. Could possibly be improved by letting a lift iterate through the order queues of every active lift before updating its own order queue.
- Ordercontroller keeps track of the statuses of all lifts and detects that a lift has left the network. It's intuitive that it should rather be the network module that detects that a lift has left the network. This detection could have notified the orderController. But, the detection of an inactive lift needs access to the timestamp of the status messages and the orderController needs access to the statuses of the all lifts in order to run the cost function. The orderController cannot access the network module as the network module needs to access the orderController (avoiding a cyclic import). Hence, the statuses cannot be saved in the network module with the current dependency between the network module and the orderController module.
 - Instead of changing the interface between these modules, there is an alternative solution; the functionality related to the saving of the statuses of all lifts in the network could be separated into a new "status" module. This decision involves a tradeoff between the size of the system, with a growing number of modules, and less module responsibility. 
