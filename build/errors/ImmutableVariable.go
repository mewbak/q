package errors

import "fmt"

// ImmutableVariable represents attempts of assigning a new value to an immutable variable.
type ImmutableVariable struct {
	VariableName string
}

func (err *ImmutableVariable) Error() string {
	return fmt.Sprintf("Variable '%s' can not be modified (make it mutable via 'mut %s' on first usage)", err.VariableName, err.VariableName)
}
