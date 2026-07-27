package main

import (
	"fmt"
	"os"
)

func leb128U32(v uint32) []byte {
	var result []byte
	for {
		b := byte(v & 0x7f)
		v >>= 7
		if v != 0 {
			b |= 0x80
		}
		result = append(result, b)
		if v == 0 {
			break
		}
	}
	return result
}

func leb128S64(v int64) []byte {
	var result []byte
	for {
		b := byte(v & 0x7f)
		v >>= 7
		if (v == 0 && (b&0x40) == 0) || (v == -1 && (b&0x40) != 0) {
			result = append(result, b)
			break
		}
		b |= 0x80
		result = append(result, b)
	}
	return result
}

func section(id byte, content []byte) []byte {
	var s []byte
	s = append(s, id)
	s = append(s, leb128U32(uint32(len(content)))...)
	s = append(s, content...)
	return s
}

func exportEntry(name string, kind byte, index uint32) []byte {
	var e []byte
	e = append(e, leb128U32(uint32(len(name)))...)
	e = append(e, []byte(name)...)
	e = append(e, kind)
	e = append(e, leb128U32(index)...)
	return e
}

func buildEchoWASM() []byte {
	var wasm []byte

	wasm = append(wasm, 0x00, 0x61, 0x73, 0x6d)
	wasm = append(wasm, 0x01, 0x00, 0x00, 0x00)

	var typeSection []byte
	typeSection = append(typeSection, leb128U32(3)...)

	typeSection = append(typeSection, 0x60)
	typeSection = append(typeSection, leb128U32(1)...)
	typeSection = append(typeSection, 0x7f)
	typeSection = append(typeSection, leb128U32(1)...)
	typeSection = append(typeSection, 0x7f)

	typeSection = append(typeSection, 0x60)
	typeSection = append(typeSection, leb128U32(2)...)
	typeSection = append(typeSection, 0x7f, 0x7f)
	typeSection = append(typeSection, leb128U32(0)...)

	typeSection = append(typeSection, 0x60)
	typeSection = append(typeSection, leb128U32(2)...)
	typeSection = append(typeSection, 0x7f, 0x7f)
	typeSection = append(typeSection, leb128U32(1)...)
	typeSection = append(typeSection, 0x7e)

	wasm = append(wasm, section(0x01, typeSection)...)

	var funcSection []byte
	funcSection = append(funcSection, leb128U32(3)...)
	funcSection = append(funcSection, leb128U32(0)...)
	funcSection = append(funcSection, leb128U32(1)...)
	funcSection = append(funcSection, leb128U32(2)...)
	wasm = append(wasm, section(0x03, funcSection)...)

	var memSection []byte
	memSection = append(memSection, leb128U32(1)...)
	memSection = append(memSection, 0x00)
	memSection = append(memSection, leb128U32(1)...)
	wasm = append(wasm, section(0x05, memSection)...)

	var globalSection []byte
	globalSection = append(globalSection, leb128U32(1)...)
	globalSection = append(globalSection, 0x7f)
	globalSection = append(globalSection, 0x01)
	globalSection = append(globalSection, 0x41)
	globalSection = append(globalSection, leb128S64(4096)...)
	globalSection = append(globalSection, 0x0b)
	wasm = append(wasm, section(0x06, globalSection)...)

	var exportSection []byte
	exportSection = append(exportSection, leb128U32(4)...)
	exportSection = append(exportSection, exportEntry("memory", 0x02, 0)...)
	exportSection = append(exportSection, exportEntry("amitia_alloc", 0x00, 0)...)
	exportSection = append(exportSection, exportEntry("amitia_dealloc", 0x00, 1)...)
	exportSection = append(exportSection, exportEntry("amitia_invoke", 0x00, 2)...)
	wasm = append(wasm, section(0x07, exportSection)...)

	var codeSection []byte
	codeSection = append(codeSection, leb128U32(3)...)

	func0Body := []byte{
		0x01, 0x01, 0x7f,
		0x23, 0x00,
		0x21, 0x01,
		0x23, 0x00,
		0x20, 0x00,
		0x6a,
		0x24, 0x00,
		0x20, 0x01,
		0x0b,
	}
	codeSection = append(codeSection, leb128U32(uint32(len(func0Body)))...)
	codeSection = append(codeSection, func0Body...)

	func1Body := []byte{
		0x00,
		0x0b,
	}
	codeSection = append(codeSection, leb128U32(uint32(len(func1Body)))...)
	codeSection = append(codeSection, func1Body...)

	func2Body := []byte{
		0x01, 0x03, 0x7f,
		0x23, 0x00,
		0x21, 0x02,
		0x23, 0x00,
		0x20, 0x01,
		0x6a,
		0x24, 0x00,
		0x41, 0x00,
		0x21, 0x03,
		0x02, 0x40,
		0x03, 0x40,
		0x20, 0x03,
		0x20, 0x01,
		0x4e,
		0x0d, 0x01,
		0x20, 0x00,
		0x20, 0x03,
		0x6a,
		0x2d, 0x00, 0x00,
		0x21, 0x04,
		0x20, 0x02,
		0x20, 0x03,
		0x6a,
		0x20, 0x04,
		0x3a, 0x00, 0x00,
		0x20, 0x03,
		0x41, 0x01,
		0x6a,
		0x21, 0x03,
		0x0c, 0x00,
		0x0b,
		0x0b,
		0x20, 0x02,
		0xad,
		0x42, 0x20,
		0x86,
		0x20, 0x01,
		0xad,
		0x84,
		0x0b,
	}
	codeSection = append(codeSection, leb128U32(uint32(len(func2Body)))...)
	codeSection = append(codeSection, func2Body...)

	wasm = append(wasm, section(0x0a, codeSection)...)

	return wasm
}

func main() {
	wasm := buildEchoWASM()

	outputPath := "echo_prebuilt.wasm"
	if len(os.Args) > 1 {
		outputPath = os.Args[1]
	}

	if err := os.WriteFile(outputPath, wasm, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", outputPath, err)
		os.Exit(1)
	}

	fmt.Printf("Generated %s (%d bytes)\n", outputPath, len(wasm))
}
