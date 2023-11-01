package assembler

import (
	"strconv"
	"strings"
)

func (r *AssembledResult) GetLineOfAddress(address uint32, offset uint32) int {
	return r.AddressToLine[address-offset] + 1
}

func (r *AssembledResult) GetAddressOfLine(line int) uint32 {
	for addr, l := range r.AddressToLine {
		if l == line-1 {
			return addr
		}
	}

	return 0xFFFFFFFF
}

// Returns the nearest label for the line of the given address
// only looks upwards (lower memory addresses) for labels in the code
func (r *AssembledResult) GetTextLabelForAddress(address uint32) string {
	closestLabel := ""
	closestAddrDelta := uint32(0xFFFFFFFF)
	for label, addr := range r.Labels {
		if addr <= address && address-addr < closestAddrDelta && r.LabelTypes[label] == "text" {
			closestLabel = label
			closestAddrDelta = address - addr
		}
	}

	return closestLabel
}

func (r *AssembledResult) PrettyPrintInstruction(address uint32) string {
	// should be in the form of: fileName:lineNumber label
	if _, ok := r.AddressToLine[address]; !ok {
		return "??? Unknown location"
	}

	return r.FileName + ":" + strconv.Itoa(r.AddressToLine[address]+1) + " " + r.GetTextLabelForAddress(address)
}

func (r *AssembledResult) PrettyPrintStacktrace(trace []uint32) string {
	lines := []string{}
	for _, addr := range trace {
		lines = append(lines, r.PrettyPrintInstruction(addr))
	}
	return strings.Join(lines, "\n")
}
