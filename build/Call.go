package build

import (
	"fmt"
	"sync/atomic"

	"github.com/akyoto/asm/syscall"
	"github.com/akyoto/q/build/errors"
	"github.com/akyoto/q/build/expression"
	"github.com/akyoto/q/build/register"
	"github.com/akyoto/q/build/token"
)

// Call represents a function call in the source code.
type Call struct {
	Function   *Function
	Parameters [][]token.Token
}

// CallExpression executes a function call.
func (state *State) CallExpression(expr *expression.Expression) error {
	functionName := expr.Token.Text()
	function := state.environment.Functions[functionName]
	isBuiltin := false

	if function == nil {
		function = BuiltinFunctions[functionName]
		isBuiltin = true
	}

	if function == nil {
		return state.UnknownFunctionError(functionName)
	}

	parameters := expr.Children

	// Calling a function with side effects causes our function to have side effects
	if atomic.LoadInt32(&function.SideEffects) > 0 {
		atomic.AddInt32(&state.function.SideEffects, 1)
	}

	// Parameter check
	if !function.NoParameterCheck && len(parameters) != len(function.Parameters) {
		return &errors.ParameterCount{
			FunctionName:  function.Name,
			CountGiven:    len(parameters),
			CountRequired: len(function.Parameters),
		}
	}

	// print is a little special
	if isBuiltin && functionName == "print" {
		parameter := parameters[0]

		if parameter.Token.Kind != token.Text {
			return fmt.Errorf("'%s' requires a text parameter instead of '%s'", function.Name, parameter.Token.Text())
		}

		text := parameter.Token.Text() + "\n"
		address := state.assembler.AddString(text)
		state.assembler.MoveRegisterNumber(state.registers.Syscall[0], uint64(syscall.Write))
		state.assembler.MoveRegisterNumber(state.registers.Syscall[1], 1)
		state.assembler.MoveRegisterAddress(state.registers.Syscall[2], address)
		state.assembler.MoveRegisterNumber(state.registers.Syscall[3], uint64(len(text)))
		state.assembler.Syscall()
		return nil
	}

	// Call the function
	err := state.BeforeCall(parameters)

	if err != nil {
		return err
	}

	if functionName == "syscall" {
		state.assembler.Syscall()
	} else {
		state.assembler.Call(functionName)
	}

	state.AfterCall(function)

	// Free the call registers
	for _, callRegister := range state.registers.Call {
		callRegister.Free()
	}

	// Mark return value register temporarily as used for better assembly output
	returnValueRegister := state.registers.ReturnValue[0]
	err = returnValueRegister.Use(expr)

	if err != nil {
		return err
	}

	// Save return value in temporary register
	if expr.Register != returnValueRegister {
		if expr.Register != nil {
			state.assembler.MoveRegisterRegister(expr.Register, returnValueRegister)
		}

		returnValueRegister.Free()
	}

	return nil
}

// Call handles function calls.
func (state *State) Call(tokens []token.Token) error {
	firstToken := tokens[0]

	if firstToken.Kind != token.Identifier {
		return errors.MissingFunctionName
	}

	lastToken := tokens[len(tokens)-1]

	if lastToken.Kind != token.GroupEnd {
		return &errors.MissingCharacter{Character: ")"}
	}

	return state.TokensToRegister(tokens, nil)
}

// BeforeCall pushes parameters into registers.
func (state *State) BeforeCall(parameters []*expression.Expression) error {
	// state.assembler.PushRegister(state.registers.RegisterByName("rbx"))

	for i, parameter := range parameters {
		callRegister := state.registers.Call[i]
		err := callRegister.Use(parameter)

		// If one of the call registers is already in use,
		// move the current user of the register to another one.
		if err != nil {
			freeRegister := state.registers.FindFreeRegister()

			if freeRegister == nil {
				return errors.ExceededMaxVariables
			}

			state.assembler.MoveRegisterRegister(freeRegister, callRegister)

			err := err.(*register.ErrAlreadyInUse)
			variable, isVariable := err.UsedBy.(*Variable)

			if isVariable {
				_ = variable.SetRegister(freeRegister)
			} else {
				panic("This should never happen")
			}

			callRegister.Free()
			_ = callRegister.Use(parameter)
		}

		// Save the parameter in the call register
		err = state.ExpressionToRegister(parameter, callRegister)

		if err != nil {
			return err
		}
	}

	return nil
}

// AfterCall restores saved registers from the stack.
func (state *State) AfterCall(function *Function) {
	atomic.AddInt32(&function.CallCount, 1)
	// state.assembler.PopRegister(state.registers.RegisterByName("rbx"))
}
