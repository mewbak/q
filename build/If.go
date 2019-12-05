package build

import (
	"fmt"

	"github.com/akyoto/q/build/errors"
	"github.com/akyoto/q/build/token"
)

// IfState handles the state of branch compilation.
type IfState struct {
	counter int
	labels  []string
}

// IfStart handles the start of if conditions.
func (state *State) IfStart(tokens []token.Token) error {
	state.Expect(token.Keyword)
	condition := tokens[1:]

	state.ifState.counter++
	elseLabel := fmt.Sprintf("if_%d_end", state.ifState.counter)
	state.ifState.labels = append(state.ifState.labels, elseLabel)

	return state.Condition(condition, elseLabel)
}

// Condition encodes a compare instruction for the given condition.
func (state *State) Condition(condition []token.Token, elseLabel string) error {
	variableName := condition[0].Text()
	variable := state.scopes.Get(variableName)

	if variable == nil {
		return &errors.UnknownVariable{VariableName: variableName}
	}

	state.UseVariable(variable)
	temporary, err := state.CompareRegisterExpression(variable.Register(), condition[2:], "")

	if err != nil {
		return err
	}

	if temporary != nil {
		temporary.Free()
	}

	operator := condition[1].Text()
	state.IfFalseJump(operator, elseLabel)
	return nil
}

// IfFalseJump jumps if the previous compare statement was false.
func (state *State) IfFalseJump(operator string, label string) {
	switch operator {
	case ">=":
		state.assembler.JumpIfLess(label)

	case ">":
		state.assembler.JumpIfLessOrEqual(label)

	case "<=":
		state.assembler.JumpIfGreater(label)

	case "<":
		state.assembler.JumpIfGreaterOrEqual(label)

	case "==":
		state.assembler.JumpIfNotEqual(label)

	case "!=":
		state.assembler.JumpIfEqual(label)
	}
}

// IfEnd handles the end of if conditions.
func (state *State) IfEnd() error {
	label := state.ifState.labels[len(state.ifState.labels)-1]
	state.ifState.labels = state.ifState.labels[:len(state.ifState.labels)-1]
	state.assembler.AddLabel(label)
	return nil
}
