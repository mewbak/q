package compiler

import (
	"sync"

	"github.com/akyoto/asm"
	"github.com/akyoto/q/token"
)

var (
	// Pool for assemblers
	assemblerPool = sync.Pool{
		New: func() interface{} {
			return asm.New()
		},
	}

	// Pool for function calls
	functionCallPool = sync.Pool{
		New: func() interface{} {
			return &FunctionCall{}
		},
	}

	// Pool for token buffers
	tokenBufferPool = sync.Pool{
		New: func() interface{} {
			buffer := make([]token.Token, 0, 1024)
			return &buffer
		},
	}
)
