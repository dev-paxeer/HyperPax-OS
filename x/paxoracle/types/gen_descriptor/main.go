// +build ignore

// This program generates the gzipped FileDescriptorProto bytes for paxoracle/tx.proto.
// Run: go run x/paxoracle/types/gen_descriptor/main.go
package main

import (
	"bytes"
	"compress/gzip"
	"fmt"

	"google.golang.org/protobuf/proto"
	descriptorpb "google.golang.org/protobuf/types/descriptorpb"
)

func main() {
	// Build a minimal FileDescriptorProto for paxoracle/tx.proto
	syntax := "proto3"
	pkg := "paxoracle"
	fileName := "paxoracle/tx.proto"

	// Field types
	typeString := descriptorpb.FieldDescriptorProto_TYPE_STRING
	typeBytes := descriptorpb.FieldDescriptorProto_TYPE_BYTES

	labelOptional := descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL

	fieldNum1 := int32(1)
	fieldNum2 := int32(2)
	fieldNum3 := int32(3)
	fieldNum4 := int32(4)

	signerName := "signer"
	marketIdName := "market_id"
	priceName := "price"
	confidenceName := "confidence"

	fd := &descriptorpb.FileDescriptorProto{
		Name:    &fileName,
		Package: &pkg,
		Syntax:  &syntax,
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: strPtr("MsgSubmitPrice"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{Name: &signerName, Number: &fieldNum1, Type: &typeString, Label: &labelOptional},
					{Name: &marketIdName, Number: &fieldNum2, Type: &typeBytes, Label: &labelOptional},
					{Name: &priceName, Number: &fieldNum3, Type: &typeBytes, Label: &labelOptional},
					{Name: &confidenceName, Number: &fieldNum4, Type: &typeBytes, Label: &labelOptional},
				},
			},
		},
	}

	b, err := proto.Marshal(fd)
	if err != nil {
		panic(err)
	}

	var buf bytes.Buffer
	gz, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	gz.Write(b)
	gz.Close()

	fmt.Println("// Gzipped FileDescriptorProto for paxoracle/tx.proto")
	fmt.Printf("var fileDescriptor_paxoracle_tx = []byte{\n\t")
	for i, byt := range buf.Bytes() {
		if i > 0 && i%16 == 0 {
			fmt.Printf("\n\t")
		}
		fmt.Printf("0x%02x, ", byt)
	}
	fmt.Println("\n}")
}

func strPtr(s string) *string { return &s }
