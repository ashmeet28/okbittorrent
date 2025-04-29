package main

import (
	"bytes"
	"errors"
	"math"
	"slices"
	"strconv"
)

func encodeValue(v any) ([]byte, error) {
	switch v.(type) {
	case map[string]any:
		return encodeDictionary(v)
	case []any:
		return encodeList(v)
	case int64:
		return encodeInt64(v)
	case []byte:
		return encodeString(v)
	default:
		return nil, errors.New("invalid value")
	}
}

func encodeString(v any) ([]byte, error) {
	s, ok := v.([]byte)
	if !ok {
		return nil, errors.New("invalid value")
	}

	var data []byte

	data = append(data, []byte(strconv.FormatInt(int64(len(s)), 10))...)
	data = append(data, 0x3a)
	data = append(data, s...)

	return data, nil
}

func encodeInt64(v any) ([]byte, error) {
	i, ok := v.(int64)
	if !ok {
		return nil, errors.New("invalid value")
	}

	var data []byte

	data = append(data, 0x69)
	data = append(data, []byte(strconv.FormatInt(i, 10))...)
	data = append(data, 0x65)

	return data, nil
}

func encodeList(v any) ([]byte, error) {
	l, ok := v.([]any)
	if !ok {
		return nil, errors.New("invalid value")
	}

	var data []byte

	data = append(data, 0x6c)
	for _, v := range l {
		valueData, err := encodeValue(v)
		if err != nil {
			return valueData, err
		}
		data = append(data, valueData...)
	}
	data = append(data, 0x65)

	return data, nil
}

func encodeDictionary(v any) ([]byte, error) {
	d, ok := v.(map[string]any)
	if !ok {
		return nil, errors.New("invalid value")
	}

	var data []byte

	data = append(data, 0x64)
	var keys []string
	for k := range d {
		for _, c := range k {
			if c < 0x20 || c > 0x7e {
				return nil, errors.New("only printable ascii characters are allowed in the key")
			}
		}
		keys = append(keys, k)
	}
	slices.SortFunc(keys, func(a, b string) int {
		return bytes.Compare([]byte(a), []byte(b))
	})
	for _, k := range keys {
		if valueData, err := encodeValue([]byte(k)); err == nil {
			data = append(data, valueData...)
		} else {
			return valueData, err
		}
		if valueData, err := encodeValue(d[k]); err == nil {
			data = append(data, valueData...)
		} else {
			return valueData, err
		}
	}
	data = append(data, 0x65)

	return data, nil
}

func BencodeEncode(v any) ([]byte, error) {
	return encodeValue(v)
}

func decodeString(data []byte) (any, []byte, error) {
	if len(data) < 2 {
		return nil, nil, errors.New("buffer too small")
	}

	colonIndex := bytes.IndexByte(data[1:], 0x3a)
	if colonIndex == -1 {
		return nil, nil, errors.New("':' not found")
	}
	colonIndex += 1

	for _, b := range data[:colonIndex] {
		if b < 0x30 || b > 0x39 {
			return nil, nil, errors.New("unable to parse the size")
		}
	}

	if data[0] == 0x30 && colonIndex != 1 {
		return nil, nil, errors.New("leading zeros are not allowed in the size")
	}

	l, err := strconv.ParseInt(string(data[:colonIndex]), 10, 64)
	if err != nil {
		return nil, nil, err
	}

	data = data[colonIndex+1:]
	if l > math.MaxInt || int(l) > len(data) {
		return nil, nil, errors.New("given size exceeds buffer length")
	}

	s := make([]byte, l)
	copy(s, data)
	return s, data[l:], nil
}

func decodeInt64(data []byte) (any, []byte, error) {
	if len(data) < 3 {
		return nil, nil, errors.New("buffer too small")
	}

	if data[0] != 0x69 {
		return nil, nil, errors.New("'i' not found at the start of the buffer")
	}
	data = data[1:]

	eIndex := bytes.IndexByte(data[1:], 0x65)
	if eIndex == -1 {
		return nil, nil, errors.New("'e' not found")
	}
	eIndex += 1

	if data[0] == 0x2d {
		if eIndex < 2 {
			return nil, nil, errors.New("unable to parse the value")
		}
		for _, b := range data[1:eIndex] {
			if b < 0x30 || b > 0x39 {
				return nil, nil, errors.New("unable to parse the value")
			}
		}
	} else {
		for _, b := range data[:eIndex] {
			if b < 0x30 || b > 0x39 {
				return nil, nil, errors.New("unable to parse the value")
			}
		}
	}

	v, err := strconv.ParseInt(string(data[:eIndex]), 10, 64)
	if err != nil {
		return nil, nil, err
	}

	if data[0] == 0x2d {
		if data[1] == 0x30 {
			return nil, nil, errors.New("leading zeros are not allowed after a minus sign")
		}
	} else if data[0] == 0x30 && eIndex != 1 {
		return nil, nil, errors.New("leading zeros are not allowed")
	}

	return v, data[eIndex+1:], nil
}

func decodeList(data []byte) (any, []byte, error) {
	if len(data) < 2 {
		return nil, nil, errors.New("buffer too small")
	}

	if data[0] != 0x6c {
		return nil, nil, errors.New("'l' not found at the start of the buffer")
	}
	data = data[1:]

	var l []any

	if data[1] == 0x65 {
		data = data[1:]
		return l, data, nil
	}

	for len(data) > 0 {
		v, dataLeft, err := decodeValue(data)
		if err != nil {
			return v, dataLeft, err
		}

		data = dataLeft
		l = append(l, v)

		if len(data) > 0 && data[0] == 0x65 {
			data = data[1:]
			return l, data, nil
		}
	}

	return nil, nil, errors.New("'e' not found")
}

func decodeDictionary(data []byte) (any, []byte, error) {
	if len(data) < 2 {
		return nil, nil, errors.New("buffer too small")
	}

	if data[0] != 0x64 {
		return nil, nil, errors.New("'d' not found at the start of the buffer")
	}
	data = data[1:]

	d := make(map[string]any)

	if data[1] == 0x65 {
		data = data[1:]
		return d, data, nil
	}

	lastKey := ""
	isLastKeyInit := false

	for len(data) > 0 {
		v, dataLeft, err := decodeValue(data)
		if err != nil {
			return v, dataLeft, err
		}
		data = dataLeft

		if k, ok := v.([]byte); ok {
			for _, c := range k {
				if c < 0x20 || c > 0x7e {
					return nil, nil,
						errors.New("only printable ascii characters are allowed in the key")
				}
			}
			if _, ok := d[string(k)]; ok {
				return nil, nil, errors.New("duplicate keys")
			}
			if isLastKeyInit {
				if bytes.Compare([]byte(lastKey), k) >= 0 {
					return nil, nil, errors.New("keys are not in sorted order")
				}
			}
			lastKey = string(k)
			isLastKeyInit = true

			if len(data) == 0 {
				return nil, nil, errors.New("value of key not found")
			}
			v, dataLeft, err := decodeValue(data)
			if err != nil {
				return v, dataLeft, err
			}
			data = dataLeft

			d[string(k)] = v
		} else {
			return nil, nil, errors.New("key is not a string")
		}

		if len(data) > 0 && data[0] == 0x65 {
			data = data[1:]
			return d, data, nil
		}
	}
	return nil, nil, errors.New("'e' not found")
}

func decodeValue(data []byte) (any, []byte, error) {
	if len(data) == 0 {
		return nil, nil, errors.New("no data to decode")
	}

	if data[0] >= 0x30 && data[0] <= 0x39 {
		return decodeString(data)
	} else if data[0] == 0x69 {
		return decodeInt64(data)
	} else if data[0] == 0x6c {
		return decodeList(data)
	} else if data[0] == 0x64 {
		return decodeDictionary(data)
	} else {
		return nil, nil, errors.New("invalid data")
	}
}

func BencodeDecode(data []byte) (any, []byte, error) {
	return decodeValue(data)
}
