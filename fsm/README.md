# fsm

Package `fsm` provides a lightweight, declarative, in-memory Finite State Machine for validating state transitions across domain entity lifecycles.

## Usage

```go
import "github.com/umesh0492/go-libs/fsm"

machine := fsm.NewMachine([]fsm.Transition{
    {From: "DRAFT", To: "PENDING", Action: "SUBMIT"},
    {From: "PENDING", To: "APPROVED", Action: "APPROVE"},
    {From: "PENDING", To: "REJECTED", Action: "REJECT"},
})

if err := machine.ValidateTransition("DRAFT", "PENDING", "SUBMIT"); err != nil {
    // Handle illegal transition
}
```
