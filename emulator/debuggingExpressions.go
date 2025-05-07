package emulator

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unsafe"

	"github.gatech.edu/ECEInnovation/RISC-V-Emulator/assembler"
)

const (
	EvaluationResultTypeInteger = iota
	EvaluationResultTypeString
	EvaluationResultTypeFloat
	EvaluationResultTypeBoolean
	EvaluationResultTypeArray
	EvaluationResultTypeError
)

type EvaluationResult struct {
	String         string
	Type           int
	isRegister     bool
	isValidAddress bool
	address        uint32
	Children       []EvaluationResult
}

type evaluationToken struct {
	dataType  string // values such as int, float, string, bool, operator
	strValue  string
	trueValue interface{}
	value     EvaluationResult
}

type evaluationFunction struct {
	name          string
	argumentNames []string
	precedence    int // 0 is highest, 1 is second highest, etc.

	// used for operators as functions
	canBeUnary     bool // only used if it can also be binary, if only unary, set false and set the appropriate unaryDirection
	unaryDirection int  // 0 for none, 1 for left, 2 for right

	// function itself
	function func([]evaluationToken) (evaluationToken, error)
}

var operators = map[string]evaluationFunction{
	"+": {
		name:          "+",
		precedence:    3,
		argumentNames: []string{"left", "right"},
		function: func(args []evaluationToken) (evaluationToken, error) {
			// only works on floats and ints
			if args[0].dataType == "int" && args[1].dataType == "int" {
				// int result
				result := args[0].trueValue.(int) + args[1].trueValue.(int)
				strRes := strconv.Itoa(result)
				return evaluationToken{
					dataType: "int",
					value: EvaluationResult{
						Type:   EvaluationResultTypeInteger,
						String: strRes,
					},
					trueValue: result,
					strValue:  strRes,
				}, nil
			} else if args[0].dataType == "float" && args[1].dataType == "float" {
				// float result
				result := args[0].trueValue.(float32) + args[1].trueValue.(float32)
				strRes := strconv.FormatFloat(float64(result), 'f', -1, 32)
				return evaluationToken{
					dataType: "float",
					value: EvaluationResult{
						Type:   EvaluationResultTypeFloat,
						String: strRes,
					},
					trueValue: result,
					strValue:  strRes,
				}, nil
			}

			return evaluationToken{}, errors.New("invalid types for operator + got " + args[0].dataType + " and " + args[1].dataType)
		},
	},
	"-": {
		name:           "-",
		precedence:     3,
		canBeUnary:     true,
		unaryDirection: 2,
		argumentNames:  []string{"left", "right"},
		function: func(args []evaluationToken) (evaluationToken, error) {
			// can be unary, in which case, negate the value.
			// otherwise, subtract the right from the left
			// works with ints and floats
			if len(args) == 1 {
				// unary operator
				if args[0].dataType == "int" {
					result := -args[0].trueValue.(int)
					strRes := strconv.Itoa(result)
					return evaluationToken{
						dataType: "int",
						value: EvaluationResult{
							Type:   EvaluationResultTypeInteger,
							String: strRes,
						},
						trueValue: result,
						strValue:  strRes,
					}, nil
				} else if args[0].dataType == "float" {
					result := -args[0].trueValue.(float32)
					strRes := strconv.FormatFloat(float64(result), 'f', -1, 32)
					return evaluationToken{
						dataType: "float",
						value: EvaluationResult{
							Type:   EvaluationResultTypeFloat,
							String: strRes,
						},
						trueValue: result,
						strValue:  strRes,
					}, nil
				} else {
					return evaluationToken{}, errors.New("invalid type for operator - got " + args[0].dataType)
				}
			} else {
				// binary operator
				if args[0].dataType == "int" && args[1].dataType == "int" {
					result := args[0].trueValue.(int) - args[1].trueValue.(int)
					strRes := strconv.Itoa(result)
					return evaluationToken{
						dataType: "int",
						value: EvaluationResult{
							Type:   EvaluationResultTypeInteger,
							String: strRes,
						},
						trueValue: result,
						strValue:  strRes,
					}, nil
				} else if args[0].dataType == "float" && args[1].dataType == "float" {
					result := args[0].trueValue.(float32) - args[1].trueValue.(float32)
					strRes := strconv.FormatFloat(float64(result), 'f', -1, 32)
					return evaluationToken{
						dataType: "float",
						value: EvaluationResult{
							Type:   EvaluationResultTypeFloat,
							String: strRes,
						},
						trueValue: result,
						strValue:  strRes,
					}, nil
				} else {
					return evaluationToken{}, errors.New("invalid types for operator - got " + args[0].dataType + " and " + args[1].dataType)
				}
			}
		},
	},
	"*": {
		name:           "*",
		precedence:     2,
		canBeUnary:     true,
		unaryDirection: 2,
		argumentNames:  []string{"left", "right"},
		function: func(args []evaluationToken) (evaluationToken, error) {
			// if unary, dereference the value
			// otherwise, multiply the left and right
			// works with ints and floats for multiplication
			if len(args) == 1 {
				// unary operator, dereference. Expecting an address
				if args[0].dataType == "int" || args[0].dataType == "int*" {
					// dereference
					address := args[0].trueValue.(int)
					result, ok := liveEmulator.memory.ReadWord(uint32(address))
					if !ok {
						return evaluationToken{}, errors.New("could not read address 0x" + strconv.FormatUint(uint64(address), 16))
					}
					strRes := strconv.Itoa(int(result))
					return evaluationToken{
						dataType: "int",
						value: EvaluationResult{
							Type:           EvaluationResultTypeInteger,
							String:         strRes,
							address:        uint32(address),
							isValidAddress: true,
						},
						trueValue: int(result),
						strValue:  strRes,
					}, nil
				} else if args[0].dataType == "float*" {
					// dereference
					address := args[0].trueValue.(int)
					memValue, ok := liveEmulator.memory.ReadWord(uint32(address))
					if !ok {
						return evaluationToken{}, errors.New("could not read address 0x" + strconv.FormatUint(uint64(address), 16))
					}
					result := *(*float32)(unsafe.Pointer(&memValue))
					strRes := strconv.FormatFloat(float64(result), 'f', -1, 32)
					return evaluationToken{
						dataType: "float",
						value: EvaluationResult{
							Type:           EvaluationResultTypeFloat,
							String:         strRes,
							address:        uint32(address),
							isValidAddress: true,
						},
						trueValue: result,
						strValue:  strRes,
					}, nil
				} else if args[0].dataType == "int16_t*" {
					// dereference
					address := args[0].trueValue.(int)
					memValue, ok := liveEmulator.memory.ReadHalfWord(uint32(address))
					if !ok {
						return evaluationToken{}, errors.New("could not read address 0x" + strconv.FormatUint(uint64(address), 16))
					}

					result := int(int16(memValue))
					strRes := strconv.Itoa(result)
					return evaluationToken{
						dataType: "int",
						value: EvaluationResult{
							Type:           EvaluationResultTypeInteger,
							String:         strRes,
							address:        uint32(address),
							isValidAddress: true,
						},
						trueValue: result,
						strValue:  strRes,
					}, nil
				} else if args[0].dataType == "char*" {
					// dereference
					address := args[0].trueValue.(int)
					memValue, ok := liveEmulator.memory.ReadByte(uint32(address))
					if !ok {
						return evaluationToken{}, errors.New("could not read address 0x" + strconv.FormatUint(uint64(address), 16))
					}

					result := int(int8(memValue))
					strRes := strconv.Itoa(result)
					return evaluationToken{
						dataType: "int",
						value: EvaluationResult{
							Type:           EvaluationResultTypeInteger,
							String:         strRes,
							address:        uint32(address),
							isValidAddress: true,
						},
						trueValue: result,
						strValue:  strRes,
					}, nil
				} else {
					return evaluationToken{}, errors.New("invalid type for operator * got " + args[0].dataType)
				}
			} else {
				// binary operator, multiply
				if args[0].dataType == "int" && args[1].dataType == "int" {
					result := args[0].trueValue.(int) * args[1].trueValue.(int)
					strRes := strconv.Itoa(result)
					return evaluationToken{
						dataType: "int",
						value: EvaluationResult{
							Type:   EvaluationResultTypeInteger,
							String: strRes,
						},
						trueValue: result,
						strValue:  strRes,
					}, nil
				} else if args[0].dataType == "float" && args[1].dataType == "float" {
					result := args[0].trueValue.(float32) * args[1].trueValue.(float32)
					strRes := strconv.FormatFloat(float64(result), 'f', -1, 32)
					return evaluationToken{
						dataType: "float",
						value: EvaluationResult{
							Type:   EvaluationResultTypeFloat,
							String: strRes,
						},
						trueValue: result,
						strValue:  strRes,
					}, nil
				} else {
					return evaluationToken{}, errors.New("invalid types for operator * got " + args[0].dataType + " and " + args[1].dataType)
				}
			}
		},
	},
	"/": {
		name:          "/",
		precedence:    2,
		argumentNames: []string{"left", "right"},
		function: func(args []evaluationToken) (evaluationToken, error) {
			// division - only works with ints and floats
			if args[0].dataType == "int" && args[1].dataType == "int" {
				result := args[0].trueValue.(int) / args[1].trueValue.(int)
				strRes := strconv.Itoa(result)
				return evaluationToken{
					dataType: "int",
					value: EvaluationResult{
						Type:   EvaluationResultTypeInteger,
						String: strRes,
					},
					trueValue: result,
					strValue:  strRes,
				}, nil
			} else if args[0].dataType == "float" && args[1].dataType == "float" {
				result := args[0].trueValue.(float32) / args[1].trueValue.(float32)
				strRes := strconv.FormatFloat(float64(result), 'f', -1, 32)
				return evaluationToken{
					dataType: "float",
					value: EvaluationResult{
						Type:   EvaluationResultTypeFloat,
						String: strRes,
					},
					trueValue: result,
					strValue:  strRes,
				}, nil
			} else {
				return evaluationToken{}, errors.New("invalid types for operator / got " + args[0].dataType + " and " + args[1].dataType)
			}
		},
	},
	"%": {
		name:          "%",
		precedence:    2,
		argumentNames: []string{"left", "right"},
		function: func(args []evaluationToken) (evaluationToken, error) {
			// modulo - only works with ints
			if args[0].dataType == "int" && args[1].dataType == "int" {
				result := args[0].trueValue.(int) % args[1].trueValue.(int)
				strRes := strconv.Itoa(result)
				return evaluationToken{
					dataType: "int",
					value: EvaluationResult{
						Type:   EvaluationResultTypeInteger,
						String: strRes,
					},
					trueValue: result,
					strValue:  strRes,
				}, nil
			} else {
				return evaluationToken{}, errors.New("invalid types for operator % got " + args[0].dataType + " and " + args[1].dataType)
			}
		},
	},
	"&": {
		name:           "&",
		precedence:     1,
		canBeUnary:     true,
		unaryDirection: 2,
		argumentNames:  []string{"left", "right"},
		function: func(args []evaluationToken) (evaluationToken, error) {
			// can be unary or binary
			// when unary, get the address of the value
			// when binary, get the bitwise and of the two values
			if len(args) == 1 {
				if !args[0].value.isValidAddress {
					return evaluationToken{}, errors.New("unary & used on a value that has no address")
				}

				return evaluationToken{
					dataType: "int",
					value: EvaluationResult{
						Type:   EvaluationResultTypeInteger,
						String: "0x" + strconv.FormatUint(uint64(args[0].value.address), 16),
					},
					trueValue: args[0].value.address,
					strValue:  "0x" + strconv.FormatUint(uint64(args[0].value.address), 16),
				}, nil
			} else {
				// binary operator
				if args[0].dataType == "int" && args[1].dataType == "int" {
					result := args[0].trueValue.(int) & args[1].trueValue.(int)
					strRes := strconv.Itoa(result)
					return evaluationToken{
						dataType: "int",
						value: EvaluationResult{
							Type:   EvaluationResultTypeInteger,
							String: strRes,
						},
						trueValue: result,
						strValue:  strRes,
					}, nil
				} else {
					return evaluationToken{}, errors.New("invalid types for operator & got " + args[0].dataType + " and " + args[1].dataType)
				}
			}
		},
	},
	"|": {
		name:          "|",
		precedence:    9,
		argumentNames: []string{"left", "right"},
		function: func(args []evaluationToken) (evaluationToken, error) {
			// bitwise or - only works with ints
			if args[0].dataType == "int" && args[1].dataType == "int" {
				result := args[0].trueValue.(int) | args[1].trueValue.(int)
				strRes := strconv.Itoa(result)
				return evaluationToken{
					dataType: "int",
					value: EvaluationResult{
						Type:   EvaluationResultTypeInteger,
						String: strRes,
					},
					trueValue: result,
					strValue:  strRes,
				}, nil
			} else {
				return evaluationToken{}, errors.New("invalid types for operator | got " + args[0].dataType + " and " + args[1].dataType)
			}
		},
	},
	"^": {
		name:          "^",
		precedence:    8,
		argumentNames: []string{"left", "right"},
		function: func(args []evaluationToken) (evaluationToken, error) {
			// bitwise xor - only works with ints
			if args[0].dataType == "int" && args[1].dataType == "int" {
				result := args[0].trueValue.(int) ^ args[1].trueValue.(int)
				strRes := strconv.Itoa(result)
				return evaluationToken{
					dataType: "int",
					value: EvaluationResult{
						Type:   EvaluationResultTypeInteger,
						String: strRes,
					},
					trueValue: result,
					strValue:  strRes,
				}, nil
			} else {
				return evaluationToken{}, errors.New("invalid types for operator ^ got " + args[0].dataType + " and " + args[1].dataType)
			}
		},
	},
	"~": {
		name:           "~",
		precedence:     1,
		unaryDirection: 2,
		argumentNames:  []string{"operand"},
		function: func(args []evaluationToken) (evaluationToken, error) {
			// bitwise not - only works with ints
			if args[0].dataType == "int" {
				result := ^args[0].trueValue.(int)
				strRes := strconv.Itoa(result)
				return evaluationToken{
					dataType: "int",
					value: EvaluationResult{
						Type:   EvaluationResultTypeInteger,
						String: strRes,
					},
					trueValue: result,
					strValue:  strRes,
				}, nil
			} else {
				return evaluationToken{}, errors.New("invalid type for operator ~ got " + args[0].dataType)
			}
		},
	},
	"<<": {
		name:          "<<",
		precedence:    4,
		argumentNames: []string{"operand", "shiftAmt"},
		function: func(args []evaluationToken) (evaluationToken, error) {
			// bitwise left shift - only works with ints
			if args[0].dataType == "int" && args[1].dataType == "int" {
				result := args[0].trueValue.(int) << args[1].trueValue.(int)
				strRes := strconv.Itoa(result)
				return evaluationToken{
					dataType: "int",
					value: EvaluationResult{
						Type:   EvaluationResultTypeInteger,
						String: strRes,
					},
					trueValue: result,
					strValue:  strRes,
				}, nil
			} else {
				return evaluationToken{}, errors.New("invalid types for operator << got " + args[0].dataType + " and " + args[1].dataType)
			}
		},
	},
	">>": {
		name:          ">>",
		precedence:    4,
		argumentNames: []string{"operand", "shiftAmt"},
		function: func(args []evaluationToken) (evaluationToken, error) {
			// bitwise right shift - only works with ints
			if args[0].dataType == "int" && args[1].dataType == "int" {
				result := args[0].trueValue.(int) >> args[1].trueValue.(int)
				strRes := strconv.Itoa(result)
				return evaluationToken{
					dataType: "int",
					value: EvaluationResult{
						Type:   EvaluationResultTypeInteger,
						String: strRes,
					},
					trueValue: result,
					strValue:  strRes,
				}, nil
			} else {
				return evaluationToken{}, errors.New("invalid types for operator >> got " + args[0].dataType + " and " + args[1].dataType)
			}
		},
	},
	"&&": {
		name:          "&&",
		precedence:    10,
		argumentNames: []string{"left", "right"},
		function: func(args []evaluationToken) (evaluationToken, error) {
			// logical and - only works with ints
			if args[0].dataType == "int" && args[1].dataType == "int" {
				result := args[0].trueValue.(int) != 0 && args[1].trueValue.(int) != 0
				intRes := 0
				if result {
					intRes = 1
				}

				strRes := strconv.FormatBool(result)
				return evaluationToken{
					dataType: "int",
					value: EvaluationResult{
						Type:   EvaluationResultTypeBoolean,
						String: strRes,
					},
					trueValue: intRes,
					strValue:  strRes,
				}, nil
			} else {
				return evaluationToken{}, errors.New("invalid types for operator && got " + args[0].dataType + " and " + args[1].dataType)
			}
		},
	},
	"||": {
		name:          "||",
		precedence:    11,
		argumentNames: []string{"left", "right"},
		function: func(args []evaluationToken) (evaluationToken, error) {
			// logical or - only works with ints
			if args[0].dataType == "int" && args[1].dataType == "int" {
				result := args[0].trueValue.(int) != 0 || args[1].trueValue.(int) != 0
				intRes := 0
				if result {
					intRes = 1
				}

				strRes := strconv.FormatBool(result)
				return evaluationToken{
					dataType: "int",
					value: EvaluationResult{
						Type:   EvaluationResultTypeBoolean,
						String: strRes,
					},
					trueValue: intRes,
					strValue:  strRes,
				}, nil
			} else {
				return evaluationToken{}, errors.New("invalid types for operator || got " + args[0].dataType + " and " + args[1].dataType)
			}
		},
	},
	"!": {
		name:           "!",
		precedence:     1,
		unaryDirection: 2,
		argumentNames:  []string{"operand"},
		function: func(args []evaluationToken) (evaluationToken, error) {
			// logical not - only works with ints
			if args[0].dataType == "int" {
				result := args[0].trueValue.(int) == 0
				intRes := 0
				if result {
					intRes = 1
				}

				strRes := strconv.FormatBool(result)
				return evaluationToken{
					dataType: "int",
					value: EvaluationResult{
						Type:   EvaluationResultTypeBoolean,
						String: strRes,
					},
					trueValue: intRes,
					strValue:  strRes,
				}, nil
			} else {
				return evaluationToken{}, errors.New("invalid type for operator ! got " + args[0].dataType)
			}
		},
	},
	"==": {
		name:          "==",
		precedence:    7,
		argumentNames: []string{"left", "right"},
		function: func(args []evaluationToken) (evaluationToken, error) {
			// equality - works with ints for floats
			if args[0].dataType == "int" && args[1].dataType == "int" {
				result := args[0].trueValue.(int) == args[1].trueValue.(int)
				intRes := 0
				if result {
					intRes = 1
				}

				strRes := strconv.FormatBool(result)
				return evaluationToken{
					dataType: "int",
					value: EvaluationResult{
						Type:   EvaluationResultTypeBoolean,
						String: strRes,
					},
					trueValue: intRes,
					strValue:  strRes,
				}, nil
			} else if args[0].dataType == "float" && args[1].dataType == "float" {
				result := args[0].trueValue.(float32) == args[1].trueValue.(float32)
				intRes := 0
				if result {
					intRes = 1
				}

				strRes := strconv.FormatBool(result)
				return evaluationToken{
					dataType: "int",
					value: EvaluationResult{
						Type:   EvaluationResultTypeBoolean,
						String: strRes,
					},
					trueValue: intRes,
					strValue:  strRes,
				}, nil
			} else {
				return evaluationToken{}, errors.New("invalid types for operator == got " + args[0].dataType + " and " + args[1].dataType)
			}
		},
	},
	"!=": {
		name:          "!=",
		precedence:    7,
		argumentNames: []string{"left", "right"},
		function: func(args []evaluationToken) (evaluationToken, error) {
			// inequality - works with ints for floats
			if args[0].dataType == "int" && args[1].dataType == "int" {
				result := args[0].trueValue.(int) != args[1].trueValue.(int)
				intRes := 0
				if result {
					intRes = 1
				}

				strRes := strconv.FormatBool(result)
				return evaluationToken{
					dataType: "int",
					value: EvaluationResult{
						Type:   EvaluationResultTypeBoolean,
						String: strRes,
					},
					trueValue: intRes,
					strValue:  strRes,
				}, nil
			} else if args[0].dataType == "float" && args[1].dataType == "float" {
				result := args[0].trueValue.(float32) != args[1].trueValue.(float32)
				intRes := 0
				if result {
					intRes = 1
				}

				strRes := strconv.FormatBool(result)
				return evaluationToken{
					dataType: "int",
					value: EvaluationResult{
						Type:   EvaluationResultTypeBoolean,
						String: strRes,
					},
					trueValue: intRes,
					strValue:  strRes,
				}, nil
			} else {
				return evaluationToken{}, errors.New("invalid types for operator != got " + args[0].dataType + " and " + args[1].dataType)
			}
		},
	},
	"<": {
		name:          "<",
		precedence:    6,
		argumentNames: []string{"left", "right"},
		function: func(args []evaluationToken) (evaluationToken, error) {
			// less than - works with ints for floats
			if args[0].dataType == "int" && args[1].dataType == "int" {
				result := args[0].trueValue.(int) < args[1].trueValue.(int)
				intRes := 0
				if result {
					intRes = 1
				}

				strRes := strconv.FormatBool(result)
				return evaluationToken{
					dataType: "int",
					value: EvaluationResult{
						Type:   EvaluationResultTypeBoolean,
						String: strRes,
					},
					trueValue: intRes,
					strValue:  strRes,
				}, nil
			} else if args[0].dataType == "float" && args[1].dataType == "float" {
				result := args[0].trueValue.(float32) < args[1].trueValue.(float32)
				intRes := 0
				if result {
					intRes = 1
				}

				strRes := strconv.FormatBool(result)
				return evaluationToken{
					dataType: "int",
					value: EvaluationResult{
						Type:   EvaluationResultTypeBoolean,
						String: strRes,
					},
					trueValue: intRes,
					strValue:  strRes,
				}, nil
			} else {
				return evaluationToken{}, errors.New("invalid types for operator < got " + args[0].dataType + " and " + args[1].dataType)
			}
		},
	},
	"<=": {
		name:          "<=",
		precedence:    6,
		argumentNames: []string{"left", "right"},
		function: func(args []evaluationToken) (evaluationToken, error) {
			// less than or equal to - works with ints for floats
			if args[0].dataType == "int" && args[1].dataType == "int" {
				result := args[0].trueValue.(int) <= args[1].trueValue.(int)
				intRes := 0
				if result {
					intRes = 1
				}

				strRes := strconv.FormatBool(result)
				return evaluationToken{
					dataType: "int",
					value: EvaluationResult{
						Type:   EvaluationResultTypeBoolean,
						String: strRes,
					},
					trueValue: intRes,
					strValue:  strRes,
				}, nil
			} else if args[0].dataType == "float" && args[1].dataType == "float" {
				result := args[0].trueValue.(float32) <= args[1].trueValue.(float32)
				intRes := 0
				if result {
					intRes = 1
				}

				strRes := strconv.FormatBool(result)
				return evaluationToken{
					dataType: "int",
					value: EvaluationResult{
						Type:   EvaluationResultTypeBoolean,
						String: strRes,
					},
					trueValue: intRes,
					strValue:  strRes,
				}, nil
			} else {
				return evaluationToken{}, errors.New("invalid types for operator <= got " + args[0].dataType + " and " + args[1].dataType)
			}
		},
	},
	">": {
		name:          ">",
		precedence:    6,
		argumentNames: []string{"left", "right"},
		function: func(args []evaluationToken) (evaluationToken, error) {
			// greater than - works with ints for floats
			if args[0].dataType == "int" && args[1].dataType == "int" {
				result := args[0].trueValue.(int) > args[1].trueValue.(int)
				intRes := 0
				if result {
					intRes = 1
				}

				strRes := strconv.FormatBool(result)
				return evaluationToken{
					dataType: "int",
					value: EvaluationResult{
						Type:   EvaluationResultTypeBoolean,
						String: strRes,
					},
					trueValue: intRes,
					strValue:  strRes,
				}, nil
			} else if args[0].dataType == "float" && args[1].dataType == "float" {
				result := args[0].trueValue.(float32) > args[1].trueValue.(float32)
				intRes := 0
				if result {
					intRes = 1
				}

				strRes := strconv.FormatBool(result)
				return evaluationToken{
					dataType: "int",
					value: EvaluationResult{
						Type:   EvaluationResultTypeBoolean,
						String: strRes,
					},
					trueValue: intRes,
					strValue:  strRes,
				}, nil
			} else {
				return evaluationToken{}, errors.New("invalid types for operator > got " + args[0].dataType + " and " + args[1].dataType)
			}
		},
	},
	">=": {
		name:          ">=",
		precedence:    6,
		argumentNames: []string{"left", "right"},
		function: func(args []evaluationToken) (evaluationToken, error) {
			// greater than or equal to - works with ints for floats
			if args[0].dataType == "int" && args[1].dataType == "int" {
				result := args[0].trueValue.(int) >= args[1].trueValue.(int)
				intRes := 0
				if result {
					intRes = 1
				}

				strRes := strconv.FormatBool(result)
				return evaluationToken{
					dataType: "int",
					value: EvaluationResult{
						Type:   EvaluationResultTypeBoolean,
						String: strRes,
					},
					trueValue: intRes,
					strValue:  strRes,
				}, nil
			} else if args[0].dataType == "float" && args[1].dataType == "float" {
				result := args[0].trueValue.(float32) >= args[1].trueValue.(float32)
				intRes := 0
				if result {
					intRes = 1
				}

				strRes := strconv.FormatBool(result)
				return evaluationToken{
					dataType: "int",
					value: EvaluationResult{
						Type:   EvaluationResultTypeBoolean,
						String: strRes,
					},
					trueValue: intRes,
					strValue:  strRes,
				}, nil
			} else {
				return evaluationToken{}, errors.New("invalid types for operator >= got " + args[0].dataType + " and " + args[1].dataType)
			}
		},
	},
	"[]": {
		name:          "[]",
		precedence:    0,
		argumentNames: []string{"array", "index"},
		function: func(args []evaluationToken) (evaluationToken, error) {
			// array indexing
			// treats array as a pointer to the first element
			// then adds the index to the pointer
			if args[1].dataType != "int" {
				return evaluationToken{}, errors.New("invalid type for array index got " + args[1].dataType)
			}

			if args[0].dataType == "int*" || args[0].dataType == "int" {
				address := args[0].trueValue.(int) + 4*args[1].trueValue.(int)
				result, ok := liveEmulator.memory.ReadWord(uint32(address))
				if !ok {
					return evaluationToken{}, errors.New("could not read address 0x" + strconv.FormatUint(uint64(address), 16))
				}

				strRes := strconv.Itoa(int(result))
				return evaluationToken{
					dataType: "int",
					value: EvaluationResult{
						Type:           EvaluationResultTypeInteger,
						String:         strRes,
						address:        uint32(address),
						isValidAddress: true,
					},
					trueValue: int(result),
					strValue:  strRes,
				}, nil
			} else if args[0].dataType == "float*" {
				address := args[0].trueValue.(int) + 4*args[1].trueValue.(int)
				memValue, ok := liveEmulator.memory.ReadWord(uint32(address))
				if !ok {
					return evaluationToken{}, errors.New("could not read address 0x" + strconv.FormatUint(uint64(address), 16))
				}

				result := *(*float32)(unsafe.Pointer(&memValue))
				strRes := strconv.FormatFloat(float64(result), 'f', -1, 32)
				return evaluationToken{
					dataType: "float",
					value: EvaluationResult{
						Type:           EvaluationResultTypeFloat,
						String:         strRes,
						address:        uint32(address),
						isValidAddress: true,
					},
					trueValue: result,
					strValue:  strRes,
				}, nil
			} else if args[0].dataType == "int16_t*" {
				address := args[0].trueValue.(int) + 2*args[1].trueValue.(int)
				memValue, ok := liveEmulator.memory.ReadHalfWord(uint32(address))
				if !ok {
					return evaluationToken{}, errors.New("could not read address 0x" + strconv.FormatUint(uint64(address), 16))
				}

				result := int(int16(memValue))
				strRes := strconv.Itoa(result)
				return evaluationToken{
					dataType: "int",
					value: EvaluationResult{
						Type:           EvaluationResultTypeInteger,
						String:         strRes,
						address:        uint32(address),
						isValidAddress: true,
					},
					trueValue: result,
					strValue:  strRes,
				}, nil
			} else if args[0].dataType == "char*" {
				address := args[0].trueValue.(int) + args[1].trueValue.(int)
				memValue, ok := liveEmulator.memory.ReadByte(uint32(address))
				if !ok {
					return evaluationToken{}, errors.New("could not read address 0x" + strconv.FormatUint(uint64(address), 16))
				}

				result := int(int8(memValue))
				strRes := strconv.Itoa(result)
				return evaluationToken{
					dataType: "int",
					value: EvaluationResult{
						Type:           EvaluationResultTypeInteger,
						String:         strRes,
						address:        uint32(address),
						isValidAddress: true,
					},
					trueValue: result,
					strValue:  strRes,
				}, nil
			} else {
				return evaluationToken{}, errors.New("invalid type for array got " + args[0].dataType)
			}
		},
	},
	",": {
		name:          ",",
		precedence:    100, // the very last of operators to be performed, used in function evaluation
		argumentNames: []string{},
		function: func(args []evaluationToken) (evaluationToken, error) {
			// does nothing - the logic will effectively delete this operator at the very end of evaluating an expression
			return evaluationToken{}, nil
		},
	},
}

var functions = map[string]evaluationFunction{
	"hex": {
		name:          "hex",
		argumentNames: []string{"value"},
		function: func(args []evaluationToken) (evaluationToken, error) {
			if args[0].dataType != "int" && !strings.Contains(args[0].dataType, "*") {
				return evaluationToken{}, errors.New("invalid type for hex got " + args[0].dataType)
			}

			result := "0x" + strconv.FormatUint(uint64(args[0].trueValue.(int)), 16)
			evRes := args[0].value
			evRes.String = result

			return evaluationToken{
				dataType:  args[0].dataType,
				value:     evRes,
				trueValue: args[0].trueValue,
				strValue:  result,
			}, nil
		},
	},

	"binary": {
		name:          "binary",
		argumentNames: []string{"value"},
		function: func(args []evaluationToken) (evaluationToken, error) {
			if args[0].dataType != "int" && !strings.Contains(args[0].dataType, "*") {
				return evaluationToken{}, errors.New("invalid type for hex got " + args[0].dataType)
			}

			result := "0b" + strconv.FormatUint(uint64(args[0].trueValue.(int)), 2)
			evRes := args[0].value
			evRes.String = result

			return evaluationToken{
				dataType:  args[0].dataType,
				value:     evRes,
				trueValue: args[0].trueValue,
				strValue:  result,
			}, nil
		},
	},
}

func EvaluateExpression(str string) (EvaluationResult, error) {
	res, err := evaluateExpressionInternal(str)
	if err != nil {
		return EvaluationResult{}, err
	}
	return res[0].value, err
}

func evaluateExpressionInternal(str string) ([]evaluationToken, error) {
	// tokenize str
	tokens := []evaluationToken{}

	// tokenize as follows: split by operators. The map of operators can be used as an exhaustive list.
	// inside parenthesis, tokenize recursively splitting by comma
	// tokenize all literals, including string literals

	isInStringLiteral := false
	isInCharLiteral := false
	parenthesisDepth := 0
	bracketsDepth := 0
	searchStart := 0

	builder := strings.Builder{}
	for i := 0; i < len(str); i++ {
		if isInStringLiteral {
			if str[i] == '"' && (i == 0 || str[i-1] != '\\') && parenthesisDepth == 0 && bracketsDepth == 0 {
				isInStringLiteral = false
				tokens = append(tokens, evaluationToken{
					dataType: "string", // not a char* because this string is not in memory, it is just for function arguments
					value: EvaluationResult{
						Type:   EvaluationResultTypeString,
						String: builder.String(),
					},
					trueValue: unescapeString(builder.String()),
					strValue:  builder.String(),
				})

				builder.Reset()
				continue
			} else if str[i] == '"' && (i == 0 || str[i-1] != '\\') {
				isInStringLiteral = false
				continue
			}

			builder.WriteByte(str[i])
			continue
		} else if isInCharLiteral {
			if str[i] == '\'' && (i == 0 || str[i-1] != '\\') && parenthesisDepth == 0 && bracketsDepth == 0 {
				isInCharLiteral = false
				unescaped := unescapeString(builder.String())
				if len(unescaped) != 1 {
					return nil, errors.New("invalid char literal " + builder.String())
				}

				tokens = append(tokens, evaluationToken{
					dataType: "int",
					value: EvaluationResult{
						Type:   EvaluationResultTypeInteger,
						String: builder.String(),
					},
					trueValue: int(int8(unescaped[0])),
					strValue:  builder.String(),
				})

				builder.Reset()
				continue
			} else if str[i] == '\'' && (i == 0 || str[i-1] != '\\') {
				isInCharLiteral = false
				continue
			}

			builder.WriteByte(str[i])
			continue
		}

		if str[i] == '"' {
			isInStringLiteral = true
			continue
		} else if str[i] == '\'' {
			isInCharLiteral = true
			continue
		}

		if parenthesisDepth > 0 {
			if str[i] == '(' {
				parenthesisDepth++
			} else if str[i] == ')' {
				parenthesisDepth--

				if parenthesisDepth == 0 {
					// end of parenthesis
					recursiveRes, err := evaluateExpressionInternal(str[searchStart:i])
					if err != nil {
						return nil, err
					}

					tokens = append(tokens, recursiveRes...)
				}
			}

			continue
		} else if str[i] == '(' {
			if builder.Len() > 0 {
				literal, err := getLiteralEvaluationToken(builder.String())
				if err != nil {
					return nil, err
				}

				tokens = append(tokens, literal)
				builder.Reset()
			}

			// must find the other parenthesis
			parenthesisDepth++
			searchStart = i + 1
			continue
		}

		if bracketsDepth > 0 {
			if str[i] == '[' {
				bracketsDepth++
			} else if str[i] == ']' {
				bracketsDepth--

				if bracketsDepth == 0 {
					// end of brackets
					recursiveRes, err := evaluateExpressionInternal(str[searchStart:i])
					if err != nil {
						return nil, err
					}

					tokens = append(tokens, evaluationToken{
						dataType: "operator",
						value: EvaluationResult{
							Type:   EvaluationResultTypeError,
							String: "expected array index, got none",
						},
						trueValue: operators["[]"],
						strValue:  "[]",
					})
					tokens = append(tokens, recursiveRes...)
				}
			}

			continue
		} else if str[i] == '[' {
			if builder.Len() > 0 {
				literal, err := getLiteralEvaluationToken(builder.String())
				if err != nil {
					return nil, err
				}

				tokens = append(tokens, literal)
				builder.Reset()
			}

			// must find the other parenthesis
			bracketsDepth++
			searchStart = i + 1
			continue
		}

		if str[i] == ' ' || str[i] == '\t' || str[i] == '\n' || str[i] == '\r' {
			continue
		}

		if len(str) > i+1 {
			if op, ok := operators[str[i:i+2]]; ok {
				if builder.Len() > 0 {
					literal, err := getLiteralEvaluationToken(builder.String())
					if err != nil {
						return nil, err
					}

					tokens = append(tokens, literal)
					builder.Reset()
				}

				tokens = append(tokens, evaluationToken{
					dataType: "operator",
					value: EvaluationResult{
						Type:   EvaluationResultTypeError,
						String: "expected arguments for operator " + op.name + ", got none",
					},
					trueValue: op,
					strValue:  op.name,
				})

				i++
				continue
			}
		}

		if op, ok := operators[str[i:i+1]]; ok {
			if builder.Len() > 0 {
				literal, err := getLiteralEvaluationToken(builder.String())
				if err != nil {
					return nil, err
				}

				tokens = append(tokens, literal)
				builder.Reset()
			}

			tokens = append(tokens, evaluationToken{
				dataType: "operator",
				value: EvaluationResult{
					Type:   EvaluationResultTypeError,
					String: "expected arguments for operator " + op.name + ", got none",
				},
				trueValue: op,
				strValue:  op.name,
			})

			continue
		}

		builder.WriteByte(str[i])
	}

	if builder.Len() > 0 {
		literal, err := getLiteralEvaluationToken(builder.String())
		if err != nil {
			return nil, err
		}

		tokens = append(tokens, literal)
		builder.Reset()
	}

	// sendOutput("tokens: "+fmt.Sprint(tokens), true)

	// evaluate tokens
	// needs to create groups and separate them by comma operator
	tokenSets := [][]evaluationToken{}
	currentSet := []evaluationToken{}
	for _, token := range tokens {
		if token.dataType == "operator" && token.trueValue.(evaluationFunction).name == "," {
			tokenSets = append(tokenSets, currentSet)
			currentSet = []evaluationToken{}
		} else {
			currentSet = append(currentSet, token)
		}
	}

	if len(currentSet) > 0 {
		tokenSets = append(tokenSets, currentSet)
	} else {
		return nil, errors.New("expected expression after comma, got none")
	}

	// evaluate each set
	ret := []evaluationToken{}
	for i := 0; i < len(tokenSets); i++ {
		res, err := evaluateTokens(tokenSets[i])
		if err != nil {
			return nil, err
		}

		ret = append(ret, res)
	}

	return ret, nil
}

func evaluateTokens(tokens []evaluationToken) (evaluationToken, error) {
	for precedence := 0; precedence <= 10; precedence++ {
		for pos := 0; pos < len(tokens); pos++ {
			if tokens[pos].dataType != "operator" && tokens[pos].dataType != "function" {
				continue
			}

			op := tokens[pos].trueValue.(evaluationFunction)
			if op.precedence != precedence && !(op.name == "*" && pos == 0 && precedence == 1) {
				continue
			}

			// evaluate operator (or function)
			if tokens[pos].dataType == "function" {
				// function
				if len(op.argumentNames)+pos >= len(tokens) {
					return evaluationToken{}, fmt.Errorf("expected %d arguments for function %s, got %d", len(op.argumentNames), op.name, len(tokens)-pos-1)
				}

				// evaluate function
				res, err := op.function(tokens[pos+1 : pos+1+len(op.argumentNames)])
				if err != nil {
					return evaluationToken{}, err
				}

				// replace tokens
				tokens[pos] = res
				tokens = append(tokens[:pos+1], tokens[pos+1+len(op.argumentNames):]...)
			} else if op.unaryDirection == 1 && (!op.canBeUnary || pos == len(tokens)-1) {
				// unary operator with left operand
				if pos == 0 {
					return evaluationToken{}, errors.New("expected left operand for operator " + op.name + ", got none")
				}

				// evaluate operator
				res, err := op.function([]evaluationToken{tokens[pos-1]})
				if err != nil {
					return evaluationToken{}, err
				}

				// replace tokens
				tokens[pos-1] = res
				tokens = append(tokens[:pos], tokens[pos+1:]...)
				pos--
			} else if op.unaryDirection == 2 && (!op.canBeUnary || pos == 0) {
				// unary operator with right operand
				if pos == len(tokens)-1 {
					return evaluationToken{}, errors.New("expected right operand for operator " + op.name + ", got none")
				}

				// evaluate operator
				res, err := op.function([]evaluationToken{tokens[pos+1]})
				if err != nil {
					return evaluationToken{}, err
				}

				// replace tokens
				tokens[pos] = res
				tokens = append(tokens[:pos+1], tokens[pos+2:]...)
			} else {
				// binary
				if pos == 0 || pos == len(tokens)-1 {
					return evaluationToken{}, errors.New("expected operands for operator " + op.name + ", got none")
				}

				// evaluate operator
				res, err := op.function([]evaluationToken{tokens[pos-1], tokens[pos+1]})
				if err != nil {
					return evaluationToken{}, err
				}

				// replace tokens
				tokens[pos-1] = res
				tokens = append(tokens[:pos], tokens[pos+2:]...)
				pos--
			}
		}
	}

	if len(tokens) != 1 {
		return evaluationToken{}, errors.New("could not evaluate expression - too few operators")
	} else if len(tokens) == 0 {
		panic("this should never happen - 0 tokens from expression evaluation")
	}

	return tokens[0], nil
}

func getLiteralEvaluationToken(literal string) (evaluationToken, error) {
	// try to parse as int
	intVal, err := strconv.Atoi(literal)
	if err == nil {
		return evaluationToken{
			dataType: "int",
			value: EvaluationResult{
				Type:   EvaluationResultTypeInteger,
				String: literal,
			},
			trueValue: intVal,
			strValue:  literal,
		}, nil
	}

	// try to parse as float
	floatVal, err := strconv.ParseFloat(literal, 32)
	if err == nil {
		return evaluationToken{
			dataType: "float",
			value: EvaluationResult{
				Type:   EvaluationResultTypeFloat,
				String: literal,
			},
			trueValue: float32(floatVal),
			strValue:  literal,
		}, nil
	}

	// try to parse as hex number
	if len(literal) > 2 && literal[0] == '0' && (literal[1] == 'x' || literal[1] == 'X') {
		intVal, err := strconv.ParseInt(literal[2:], 16, 32)
		if err == nil {
			return evaluationToken{
				dataType: "int",
				value: EvaluationResult{
					Type:   EvaluationResultTypeInteger,
					String: literal,
				},
				trueValue: int(intVal),
				strValue:  literal,
			}, nil
		}
	}

	// try to parse as function call
	if f, ok := functions[literal]; ok {
		return evaluationToken{
			dataType: "function",
			value: EvaluationResult{
				Type:   EvaluationResultTypeError, // this should never be used as a final answer
				String: "expected arguments for function call, got none",
			},
			trueValue: f,
			strValue:  literal,
		}, nil
	}

	// try to parse as a label
	if l, ok := liveAssembledResult.Labels[literal]; ok {
		if liveAssembledResult.LabelTypes[literal] == "text" {
			l = l + assemblyEntry // assembly entry is from the debugger file
		} else {
			l = l + liveEmulator.userGlobalPointer
		}

		return evaluationToken{
			dataType: "int",
			value: EvaluationResult{
				Type:   EvaluationResultTypeInteger,
				String: "0x" + strconv.FormatUint(uint64(l), 16),
			},
			trueValue: int(l),
			strValue:  "0x" + strconv.FormatUint(uint64(l), 16),
		}, nil
	}

	// try to parse as a register
	if r, ok := assembler.RegisterNameMap[literal]; ok {
		value := liveEmulator.registers[r]

		return evaluationToken{
			dataType: "int",
			value: EvaluationResult{
				Type:       EvaluationResultTypeInteger,
				String:     strconv.FormatInt(int64(value), 10),
				isRegister: true,
				address:    uint32(r),
			},
			trueValue: int(value),
			strValue:  strconv.FormatInt(int64(value), 16),
		}, nil
	}

	return evaluationToken{}, errors.New("could not parse literal " + literal)
}

func unescapeString(str string) string {
	// supported escape sequences are: \n, \t, \r, \", \', \\

	str = strings.ReplaceAll(str, "\\n", "\n")
	str = strings.ReplaceAll(str, "\\t", "\t")
	str = strings.ReplaceAll(str, "\\r", "\r")
	str = strings.ReplaceAll(str, "\\\"", "\"")
	str = strings.ReplaceAll(str, "\\'", "'")
	str = strings.ReplaceAll(str, "\\\\", "\\")

	return str
}
