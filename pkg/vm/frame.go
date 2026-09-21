package vm

import (
    "aethium/pkg/bytecode"
)

type Frame struct {
    cl         *ClosureObject
    ip         int
    basePointer int
    returnSlot int
    isInit     bool
    instance   Value
}

func NewFrame(cl *ClosureObject, basePointer int) *Frame {
    return &Frame{
        cl:          cl,
        ip:          0,
        basePointer: basePointer,
        returnSlot:  basePointer - 1,
    }
}

func (f *Frame) Instructions() bytecode.Instructions {
    return f.cl.Fn.Instructions
}
