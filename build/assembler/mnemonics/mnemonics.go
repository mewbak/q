package mnemonics

const (
	MOV     = "mov"
	CMP     = "cmp"
	ADD     = "add"
	SUB     = "sub"
	MUL     = "imul"
	DIV     = "idiv"
	CDQ     = "cdq"
	RET     = "ret"
	SYSCALL = "syscall"
	CALL    = "call"
	JMP     = "jmp"
	JE      = "je"
	JNE     = "jne"
	JL      = "jl"
	JLE     = "jle"
	JG      = "jg"
	JGE     = "jge"
	INC     = "inc"
	DEC     = "dec"
	PUSH    = "push"
	POP     = "pop"
	CPUID   = "cpuid"

	// Artificial
	STORE = "store"
	LOAD  = "load"
)
