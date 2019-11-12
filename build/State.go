package build

import (
	"sort"
	"strconv"

	"github.com/akyoto/asm"
	"github.com/akyoto/q/build/expression"
	"github.com/akyoto/q/build/instruction"
	"github.com/akyoto/q/build/log"
	"github.com/akyoto/q/build/register"
	"github.com/akyoto/q/build/token"
	"github.com/akyoto/stringutils/similarity"
)

// State encapsulates a compiler's state.
// Every compilation requires a fresh state.
type State struct {
	instructions []instruction.Instruction
	tokens       []token.Token
	assembler    *asm.Assembler
	scopes       *ScopeStack
	registers    *register.Manager
	function     *Function
	environment  *Environment
	tokenCursor  token.Position
	instrCursor  instruction.Position
	verbose      bool
}

// CompileInstructions compiles all instructions.
func (state *State) CompileInstructions() error {
	for index, instr := range state.instructions {
		err := state.Instruction(instr, index)

		if err != nil {
			return err
		}
	}

	return nil
}

// Instruction generates machine code for the given instruction.
func (state *State) Instruction(instr instruction.Instruction, index instruction.Position) error {
	state.tokenCursor = instr.Position
	state.instrCursor = index

	switch instr.Kind {
	case instruction.Assignment:
		return state.Assignment(instr.Tokens)

	case instruction.Call:
		return state.Call(instr.Tokens)

	case instruction.Keyword:
		return state.Keyword(instr.Tokens)

	case instruction.Invalid:
		return state.Invalid(instr.Tokens)

	default:
		return nil
	}
}

// Assignment handles assignment instructions.
func (state *State) Assignment(tokens []token.Token) error {
	left := tokens[0]

	if left.Kind != token.Identifier {
		return state.Error("Expected variable on the left side of the assignment")
	}

	variableName := left.Text()
	variable := state.scopes.Get(variableName)

	if variable == nil {
		register := state.registers.FindFreeRegister()

		if register == nil {
			return state.Errorf("Exceeded maximum limit of %d variables", len(state.registers.Registers))
		}

		variable = &Variable{
			Name:     variableName,
			Position: state.tokenCursor,
		}

		variable.BindRegister(register)
		state.scopes.Add(variable)
	}

	// Skip variable name and operator
	expressionStart := 2
	state.tokenCursor += expressionStart
	value := tokens[expressionStart:]
	return state.TokensToRegister(value, variable.Register)
}

// Call handles function calls.
func (state *State) Call(tokens []token.Token) error {
	firstToken := tokens[0]

	if firstToken.Kind != token.Identifier {
		return state.Error("Expected function name before '('")
	}

	lastToken := tokens[len(tokens)-1]

	if lastToken.Kind != token.GroupEnd {
		return state.Error("Missing closing bracket ')'")
	}

	functionName := firstToken.Text()
	function := state.environment.Functions[functionName]
	isBuiltin := false

	if function == nil {
		function = BuiltinFunctions[functionName]
		isBuiltin = true
	}

	if function == nil {
		return state.UnknownFunctionError(functionName)
	}

	call := FunctionCall{
		Function: function,
	}

	bracketPos := 1
	parameterStart := bracketPos + 1
	state.tokenCursor += bracketPos
	pos := parameterStart

	for pos < len(tokens) {
		t := tokens[pos]

		switch t.Kind {
		case token.Separator:
			if pos == parameterStart {
				return state.Error("Missing parameter")
			}

			parameterTokens := tokens[parameterStart:pos]
			call.Parameters = append(call.Parameters, parameterTokens)
			parameterStart = pos + 1

		case token.GroupEnd:
			if pos == parameterStart {
				// Call with no parameters
				break
			}

			parameterTokens := tokens[parameterStart:pos]
			call.Parameters = append(call.Parameters, parameterTokens)
			parameterStart = pos + 1
		}

		state.tokenCursor++
		pos++
	}

	// Parameter check
	if !function.NoParameterCheck {
		if len(call.Parameters) < len(call.Function.Parameters) {
			return state.Errorf("Too few arguments in '%s' call", call.Function.Name)
		}

		if len(call.Parameters) > len(call.Function.Parameters) {
			return state.Errorf("Too many arguments in '%s' call", call.Function.Name)
		}
	}

	if isBuiltin {
		switch functionName {
		case "print":
			parameter := call.Parameters[0][0]

			if parameter.Kind != token.Text {
				return state.Errorf("'%s' requires a text parameter instead of '%s'", call.Function.Name, parameter.Text())
			}

			text := parameter.Text()
			state.assembler.Println(text)

		case "syscall":
			err := state.BeforeCall(&call, state.registers.SyscallRegisters)

			if err != nil {
				return err
			}

			state.assembler.Syscall()

			if state.verbose {
				log.Asm.Println("syscall")
			}

			state.AfterCall(&call)
		}

		return nil
	}

	err := state.BeforeCall(&call, state.registers.SyscallRegisters)

	if err != nil {
		return err
	}

	state.assembler.Call(call.Function.Name)

	if state.verbose {
		log.Asm.Printf("call %s\n", call.Function.Name)
	}

	state.AfterCall(&call)
	return nil
}

// Keyword handles keywords.
func (state *State) Keyword(tokens []token.Token) error {
	return state.Error("Not implemented")
}

// Invalid handles invalid instructions.
func (state *State) Invalid(tokens []token.Token) error {
	openingBrackets := token.Count(tokens, token.GroupStart)
	closingBrackets := token.Count(tokens, token.GroupEnd)

	if openingBrackets < closingBrackets {
		return state.Error("Missing opening bracket '('")
	}

	if openingBrackets > closingBrackets {
		return state.Error("Missing closing bracket ')'")
	}

	return state.Error("Invalid instruction")
}

// BeforeCall pushes parameters into registers.
func (state *State) BeforeCall(call *FunctionCall, registers []*register.Register) error {
	for index, tokens := range call.Parameters {
		register := registers[index]
		err := state.TokensToRegister(tokens, register)

		if err != nil {
			return err
		}
	}

	return nil
}

// AfterCall restores saved registers from the stack.
func (state *State) AfterCall(call *FunctionCall) {
	call.Function.Used = true
}

// TokenToRegister moves a token into a register.
// It only works with identifiers, numbers and texts.
func (state *State) TokenToRegister(singleToken token.Token, register *register.Register) error {
	switch singleToken.Kind {
	case token.Identifier:
		variableName := singleToken.Text()
		variable := state.scopes.Get(variableName)

		if variable == nil {
			return state.Errorf("Unknown variable %s", variableName)
		}

		variable.AliveUntil = state.instrCursor + 1

		// Moving a variable into its own register is pointless
		if variable.Register == register {
			return nil
		}

		state.assembler.MoveRegisterRegister(register.Name, variable.Register.Name)

		if state.verbose {
			log.Asm.Printf("mov %s, %s\n", register, variable.Register)
		}

	case token.Number:
		numberString := singleToken.Text()
		number, err := strconv.ParseInt(numberString, 10, 64)

		if err != nil {
			return state.Errorf("Not a number: %s", numberString)
		}

		state.assembler.MoveRegisterNumber(register.Name, uint64(number))

		if state.verbose {
			log.Asm.Printf("mov %s, %d\n", register, number)
		}

	case token.Text:
		address := state.assembler.Strings.Add(singleToken.Text())
		state.assembler.MoveRegisterAddress(register.Name, address)

		if state.verbose {
			log.Asm.Printf("mov %s, <%d>\n", register, address)
		}
	}

	return nil
}

// TokensToRegister moves the result of a token expression into the given register.
func (state *State) TokensToRegister(tokens []token.Token, register *register.Register) error {
	if len(tokens) == 1 {
		return state.TokenToRegister(tokens[0], register)
	}

	expr, err := expression.FromTokens(tokens)

	if err != nil {
		return state.Error(err.Error())
	}

	return state.ExpressionToRegister(expr, register)
}

// ExpressionToRegister moves the result of an expression into the given register.
func (state *State) ExpressionToRegister(expr *expression.Expression, register *register.Register) error {
	if expr.Value.Kind != token.Operator {
		return state.TokenToRegister(expr.Value, register)
	}

	return expr.EachOperation(func(sub *expression.Expression) error {
		sub.Register = register

		// Left operand
		left := sub.Children[0]

		if left.IsLeaf() {
			err := state.TokenToRegister(left.Value, sub.Register)

			if err != nil {
				return err
			}
		} else if sub.Register != left.Register {
			state.assembler.MoveRegisterRegister(sub.Register.Name, left.Register.Name)

			if state.verbose {
				log.Asm.Printf("mov %s, %s\n", sub.Register, left.Register.Name)
			}
		}

		// Right operand
		right := sub.Children[1]

		switch sub.Value.Text() {
		case "+":
			if right.IsLeaf() {
				switch right.Value.Kind {
				case token.Identifier:
					variableName := right.Value.Text()
					variable := state.scopes.Get(variableName)

					if variable == nil {
						return state.Errorf("Unknown variable %s", variableName)
					}

					variable.AliveUntil = state.instrCursor + 1
					state.assembler.AddRegisterRegister(sub.Register.Name, variable.Register.Name)

					if state.verbose {
						log.Asm.Printf("add %s, %s\n", sub.Register, variable.Register)
					}

				case token.Number:
					number, err := strconv.ParseInt(right.Value.Text(), 10, 64)

					if err != nil {
						return state.Errorf("Not a number: %s", right.Value.Text())
					}

					state.assembler.AddRegisterNumber(sub.Register.Name, uint64(number))

					if state.verbose {
						log.Asm.Printf("add %s, %s\n", sub.Register, right.Value.Text())
					}
				}

				return nil
			}

			state.assembler.AddRegisterRegister(sub.Register.Name, right.Register.Name)

			if state.verbose {
				log.Asm.Printf("add %s, %s\n", sub.Register, right.Register.Name)
			}

		case "-":
			return state.Error("Not implemented")

		default:
			return state.Error("Not implemented")
		}

		return nil
	})
}

// Error generates an error message at the current token position.
func (state *State) Error(message string) error {
	return state.function.Error(state.tokenCursor, message)
}

// Errorf generates a formatted error message at the current token position.
func (state *State) Errorf(message string, args ...interface{}) error {
	return state.function.Errorf(state.tokenCursor, message, args...)
}

// UnknownFunctionError produces an unknown function error
// and tries to guess which function the user was trying to type.
func (state *State) UnknownFunctionError(functionName string) error {
	knownFunctions := make([]string, 0, len(state.environment.Functions)+len(BuiltinFunctions))

	for builtin := range BuiltinFunctions {
		knownFunctions = append(knownFunctions, builtin)
	}

	for function := range state.environment.Functions {
		knownFunctions = append(knownFunctions, function)
	}

	// Suggest a function name based on the similarity to known functions
	sort.Slice(knownFunctions, func(a, b int) bool {
		aSimilarity := similarity.JaroWinkler(functionName, knownFunctions[a])
		bSimilarity := similarity.JaroWinkler(functionName, knownFunctions[b])
		return aSimilarity > bSimilarity
	})

	if similarity.JaroWinkler(functionName, knownFunctions[0]) > 0.9 {
		return state.Errorf("Unknown function '%s', did you mean '%s'?", functionName, knownFunctions[0])
	}

	return state.Errorf("Unknown function '%s'", functionName)
}
