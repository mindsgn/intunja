package engine

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
)

// BencodeValue represents any bencoded value
type BencodeValue interface {
	Encode() []byte
}

// BencodeString is a byte string (can contain binary data)
type BencodeString []byte

func (s BencodeString) Encode() []byte {
	return []byte(fmt.Sprintf("%d:%s", len(s), s))
}

// BencodeInt is an integer
type BencodeInt int64

func (i BencodeInt) Encode() []byte {
	return []byte(fmt.Sprintf("i%de", i))
}

// BencodeList is an ordered list
type BencodeList []BencodeValue

func (l BencodeList) Encode() []byte {
	buf := bytes.NewBuffer([]byte("l"))
	for _, v := range l {
		buf.Write(v.Encode())
	}
	buf.WriteByte('e')
	return buf.Bytes()
}

// BencodeDict is a dictionary with string keys
type BencodeDict map[string]BencodeValue

func (d BencodeDict) Encode() []byte {
	// CRITICAL: Keys must be sorted for deterministic encoding (info-hash calculation)
	keys := make([]string, 0, len(d))
	for k := range d {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	buf := bytes.NewBuffer([]byte("d"))
	for _, k := range keys {
		buf.Write(BencodeString(k).Encode())
		buf.Write(d[k].Encode())
	}
	buf.WriteByte('e')
	return buf.Bytes()
}

// Decoder for bencoded data
type BencodeDecoder struct {
	data []byte
	pos  int
}

func NewBencodeDecoder(data []byte) *BencodeDecoder {
	return &BencodeDecoder{data: data, pos: 0}
}

func (d *BencodeDecoder) Decode() (BencodeValue, error) {
	if d.pos >= len(d.data) {
		return nil, io.EOF
	}

	switch d.data[d.pos] {
	case 'i':
		return d.decodeInt()
	case 'l':
		return d.decodeList()
	case 'd':
		return d.decodeDict()
	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return d.decodeString()
	default:
		return nil, fmt.Errorf("invalid bencode at position %d: %c", d.pos, d.data[d.pos])
	}
}

func (d *BencodeDecoder) decodeInt() (BencodeInt, error) {
	d.pos++ // skip 'i'
	start := d.pos
	for d.pos < len(d.data) && d.data[d.pos] != 'e' {
		d.pos++
	}
	if d.pos >= len(d.data) {
		return 0, errors.New("unterminated integer")
	}
	val, err := strconv.ParseInt(string(d.data[start:d.pos]), 10, 64)
	d.pos++ // skip 'e'
	return BencodeInt(val), err
}

func (d *BencodeDecoder) decodeString() (BencodeString, error) {
	start := d.pos
	for d.pos < len(d.data) && d.data[d.pos] >= '0' && d.data[d.pos] <= '9' {
		d.pos++
	}
	if d.pos >= len(d.data) || d.data[d.pos] != ':' {
		return nil, errors.New("invalid string length")
	}
	length, err := strconv.Atoi(string(d.data[start:d.pos]))
	if err != nil {
		return nil, err
	}
	d.pos++ // skip ':'
	if d.pos+length > len(d.data) {
		return nil, errors.New("string length exceeds data")
	}
	str := d.data[d.pos : d.pos+length]
	d.pos += length
	return BencodeString(str), nil
}

func (d *BencodeDecoder) decodeList() (BencodeList, error) {
	d.pos++ // skip 'l'
	list := BencodeList{}
	for d.pos < len(d.data) && d.data[d.pos] != 'e' {
		val, err := d.Decode()
		if err != nil {
			return nil, err
		}
		list = append(list, val)
	}
	if d.pos >= len(d.data) {
		return nil, errors.New("unterminated list")
	}
	d.pos++ // skip 'e'
	return list, nil
}

func (d *BencodeDecoder) decodeDict() (BencodeDict, error) {
	d.pos++ // skip 'd'
	dict := make(BencodeDict)
	for d.pos < len(d.data) && d.data[d.pos] != 'e' {
		keyVal, err := d.Decode()
		if err != nil {
			return nil, err
		}
		key, ok := keyVal.(BencodeString)
		if !ok {
			return nil, errors.New("dictionary key must be a string")
		}
		val, err := d.Decode()
		if err != nil {
			return nil, err
		}
		dict[string(key)] = val
	}
	if d.pos >= len(d.data) {
		return nil, errors.New("unterminated dictionary")
	}
	d.pos++ // skip 'e'
	return dict, nil
}
