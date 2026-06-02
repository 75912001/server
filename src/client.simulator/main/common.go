package main

import (
	"fmt"

	xpacket "github.com/75912001/xlib/packet"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

var apiYamlPath string

func marshalJSON(msg proto.Message) string {
	data, err := protojson.MarshalOptions{
		Multiline:       true,
		Indent:          "  ",
		EmitUnpopulated: true,
	}.Marshal(msg)
	if err != nil {
		return fmt.Sprintf(`{"error":"%v"}`, err)
	}
	return string(data)
}

func marshalHeaderMap(header *xpacket.Header) map[string]any {
	return map[string]any{
		"MessageID": fmt.Sprintf("0x%x", header.MessageID),
		"Length":    header.Length,
		"SessionID": header.SessionID,
		"ResultID":  header.ResultID,
		"Key":       header.Key,
	}
}
