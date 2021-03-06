package build

import "github.com/akyoto/q/build/types"

const (
	BuiltinSyscall = "syscall"
	BuiltinPrint   = "print"
	BuiltinStore   = "store"
)

// BuiltinFunctions defines the builtin functions.
var BuiltinFunctions = map[string]*Function{
	BuiltinPrint: {
		Name: BuiltinPrint,
		Parameters: []*Parameter{
			{Name: "text", Type: types.Text},
		},
		ReturnTypes: nil,
		IsBuiltin:   true,
		SideEffects: 1,
	},
	BuiltinStore: {
		Name: BuiltinStore,
		Parameters: []*Parameter{
			{Name: "ptr", Type: types.Pointer},
			{Name: "offset", Type: types.Int},
			{Name: "byteCount", Type: types.Int},
			{Name: "value", Type: types.Int},
		},
		ReturnTypes: nil,
		IsBuiltin:   true,
		SideEffects: 1,
	},
	BuiltinSyscall: {
		Name: BuiltinSyscall,
		Parameters: []*Parameter{
			{Name: "syscall number", Type: types.Int},
			{Name: "param1"},
			{Name: "param2"},
			{Name: "param3"},
			{Name: "param4"},
			{Name: "param5"},
			{Name: "param6"},
		},
		ReturnTypes:      []*types.Type{types.Int},
		NoParameterCheck: true,
		IsBuiltin:        true,
		SideEffects:      1,
	},
}
