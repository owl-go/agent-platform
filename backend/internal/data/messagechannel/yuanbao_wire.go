package messagechannel

import (
	"encoding/json"
	"unicode/utf8"

	"google.golang.org/protobuf/encoding/protowire"
)

// This is the bounded text-only subset of Tencent's published wire schema.
// Field numbers follow conn.proto and the send/inbound message schemas.
type wireFields struct {
	bytes   map[protowire.Number][][]byte
	numbers map[protowire.Number]uint64
}

func readWire(data []byte) (wireFields, error) {
	result := wireFields{bytes: map[protowire.Number][][]byte{}, numbers: map[protowire.Number]uint64{}}
	if len(data) > 65536 {
		return result, providerError("provider_payload_invalid")
	}
	for len(data) > 0 {
		field, kind, n := protowire.ConsumeTag(data)
		if n < 0 || field < 1 {
			return result, providerError("provider_payload_invalid")
		}
		data = data[n:]
		switch kind {
		case protowire.BytesType:
			value, n := protowire.ConsumeBytes(data)
			if n < 0 {
				return result, providerError("provider_payload_invalid")
			}
			result.bytes[field] = append(result.bytes[field], value)
			data = data[n:]
		case protowire.VarintType:
			value, n := protowire.ConsumeVarint(data)
			if n < 0 {
				return result, providerError("provider_payload_invalid")
			}
			result.numbers[field] = value
			data = data[n:]
		default:
			n := protowire.ConsumeFieldValue(field, kind, data)
			if n < 0 {
				return result, providerError("provider_payload_invalid")
			}
			data = data[n:]
		}
	}
	return result, nil
}
func (f wireFields) data(n protowire.Number) []byte {
	if len(f.bytes[n]) == 0 {
		return nil
	}
	return f.bytes[n][0]
}
func (f wireFields) text(n protowire.Number) string {
	value := f.data(n)
	if !utf8.Valid(value) {
		return ""
	}
	return string(value)
}
func wireBytes(data []byte, n protowire.Number, value []byte) []byte {
	data = protowire.AppendTag(data, n, protowire.BytesType)
	return protowire.AppendBytes(data, value)
}
func wireText(data []byte, n protowire.Number, value string) []byte {
	return wireBytes(data, n, []byte(value))
}
func wireNumber(data []byte, n protowire.Number, value uint64) []byte {
	data = protowire.AppendTag(data, n, protowire.VarintType)
	return protowire.AppendVarint(data, value)
}

type yuanbaoFrame struct {
	kind, status, sequence uint64
	cmd, module, id        string
	ack                    bool
	body                   []byte
}

func decodeYuanbaoFrame(data []byte) (yuanbaoFrame, error) {
	outer, err := readWire(data)
	if err != nil {
		return yuanbaoFrame{}, err
	}
	head, err := readWire(outer.data(1))
	if err != nil || len(outer.data(1)) == 0 {
		return yuanbaoFrame{}, providerError("provider_payload_invalid")
	}
	return yuanbaoFrame{kind: head.numbers[1], sequence: head.numbers[3], cmd: head.text(2), id: head.text(4), module: head.text(5), ack: head.numbers[6] != 0, status: head.numbers[10], body: outer.data(2)}, nil
}
func encodeYuanbaoFrame(f yuanbaoFrame) []byte {
	head := wireNumber(nil, 1, f.kind)
	head = wireText(head, 2, f.cmd)
	if f.sequence != 0 {
		head = wireNumber(head, 3, f.sequence)
	}
	head = wireText(head, 4, f.id)
	head = wireText(head, 5, f.module)
	if f.ack {
		head = wireNumber(head, 6, 1)
	}
	if f.status != 0 {
		head = wireNumber(head, 10, f.status)
	}
	return wireBytes(wireBytes(nil, 1, head), 2, f.body)
}

type yuanbaoElement struct {
	Type    string `json:"msg_type"`
	Content struct {
		Text string `json:"text"`
		Data string `json:"data"`
	} `json:"msg_content"`
}
type yuanbaoInbound struct {
	Command   string           `json:"callback_command"`
	From      string           `json:"from_account"`
	To        string           `json:"to_account"`
	Group     string           `json:"group_code"`
	ID        string           `json:"msg_id"`
	Timestamp int64            `json:"msg_time"`
	Body      []yuanbaoElement `json:"msg_body"`
}

func decodeYuanbaoInbound(data []byte) (yuanbaoInbound, error) {
	var e yuanbaoInbound
	if json.Valid(data) {
		if err := json.Unmarshal(data, &e); err != nil {
			return e, providerError("provider_payload_invalid")
		}
		return e, nil
	}
	fields, err := readWire(data)
	if err != nil {
		return e, err
	}
	e.Command, e.From, e.To, e.Group, e.ID = fields.text(1), fields.text(2), fields.text(3), fields.text(6), fields.text(12)
	e.Timestamp = int64(fields.numbers[10])
	for _, body := range fields.bytes[13] {
		element, err := readWire(body)
		if err != nil {
			return e, err
		}
		content, err := readWire(element.data(2))
		if err != nil {
			return e, err
		}
		item := yuanbaoElement{Type: element.text(1)}
		item.Content.Text, item.Content.Data = content.text(1), content.text(4)
		e.Body = append(e.Body, item)
	}
	return e, nil
}
func yuanbaoPushPayload(f yuanbaoFrame) ([]byte, error) {
	if json.Valid(f.body) {
		return f.body, nil
	}
	fields, err := readWire(f.body)
	if err != nil {
		return nil, err
	}
	if fields.text(1) != "" && len(fields.data(4)) > 0 {
		return fields.data(4), nil
	} // PushMsg
	if fields.numbers[1] != 0 && json.Valid(fields.data(2)) {
		return fields.data(2), nil
	} // DirectedPush
	return f.body, nil
}
