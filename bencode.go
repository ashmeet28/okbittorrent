package main

import (
	"bytes"
	"errors"
	"strconv"
)

func BencodeEncode(v any) []byte {
	return make([]byte, 0)
}

func decoderHandleString(data []byte) ([]byte, any, error) {
	var strSizeString string
	for len(data) > 0 && data[0] >= 0x30 && data[0] <= 0x39 {
		strSizeString = strSizeString + string([]byte{data[0]})
		data = data[1:]
	}

	if len(strSizeString) == 0 {
		return nil, nil, errors.New("unable to decode the size of the string")
	}
	if len(data) == 0 || data[0] != 0x3a {
		return nil, nil, errors.New("missing colon after the string size")
	}
	data = data[1:]

	strSize, err := strconv.ParseInt(strSizeString, 10, 64)
	if err != nil {
		return nil, nil, errors.New("unable to parse the size of the string")
	}

	if len(data) < int(strSize) {
		return nil, nil, errors.New("size of the string is invalid")
	}

	s := make([]byte, int(strSize))
	copy(s, data)
	data = data[strSize:]
	return data, s, nil
}

func decoderHandleInt(data []byte) ([]byte, any, error) {
	var i int
	var isINeg bool

	if len(data) == 0 || data[0] != 0x69 {
		return nil, nil, errors.New("missing i in the front of the integer")
	}
	data = data[1:]

	if len(data) > 0 && data[0] == 0x2d {
		isINeg = true
		data = data[1:]
	}

	if len(data) > 0 && data[0] == 0x30 {
		data = data[1:]
		if len(data) > 0 && data[0] == 0x65 {
			if isINeg {
				return nil, nil, errors.New("encountered negative zero")
			}
			i = 0
			data = data[1:]
			return data, i, nil
		}
		if len(data) > 0 && data[0] >= 0x30 && data[0] <= 0x39 {
			return nil, nil, errors.New("leading zero in the front of the integer")
		}
		return nil, nil, errors.New("missing e in the end of the integer")
	}

	var iStr string
	for len(data) > 0 && data[0] >= 0x30 && data[0] <= 0x39 {
		iStr = iStr + string([]byte{data[0]})
		data = data[1:]
	}

	if len(data) == 0 || data[0] != 0x65 {
		return nil, nil, errors.New("missing e in the end of the integer")
	}
	data = data[1:]

	iVal, err := strconv.ParseInt("-"+iStr, 10, 64)
	if err != nil {
		return nil, nil, errors.New("unable to parse the integer")
	}
	i = int(iVal)

	return data, i, nil
}

func decoderHandleList(data []byte) ([]byte, any, error) {
	if len(data) == 0 || data[0] != 0x6c {
		return nil, nil, errors.New("missing l in the front of the list")
	}
	data = data[1:]

	var l []any
	if len(data) > 0 && data[0] == 0x65 {
		data = data[1:]
		return data, l, nil
	}

	for len(data) > 0 {
		dataLeft, v, err := decoderHandleValue(data)
		data = dataLeft
		if err != nil {
			return data, v, err
		}
		l = append(l, v)
		if len(data) > 0 && data[0] == 0x65 {
			data = data[1:]
			return data, l, nil
		}
	}
	return nil, nil, errors.New("missing e in the end of the list")
}

func decoderHandleDictionary(data []byte) ([]byte, any, error) {
	if len(data) == 0 || data[0] != 0x64 {
		return nil, nil, errors.New("missing d in the front of the dictionary")
	}
	data = data[1:]

	d := make(map[string]any)

	lastKey := ""

	if len(data) > 0 && data[0] == 0x65 {
		data = data[1:]
		return data, d, nil
	}

	for len(data) > 0 {
		dataLeft, k, err := decoderHandleValue(data)
		data = dataLeft
		if err != nil {
			return data, k, err
		}

		if a, ok := k.([]byte); ok {
			for _, c := range a {
				if c < 0x20 || c > 0x7e {
					return nil, nil,
						errors.New("only printable ascii characters are allowed in dictionary key")
				}
			}
			if _, ok := d[string(a)]; ok {
				return nil, nil, errors.New("duplicate keys in dictionary")
			}
			if bytes.Compare(a, []byte(lastKey)) != 1 {
				return nil, nil, errors.New("keys are not in sorted order")
			}
			lastKey = string(a)

			if len(data) == 0 {
				return nil, nil, errors.New("missing value of key in dictionary")
			}
			dataLeft, v, err := decoderHandleValue(data)
			data = dataLeft
			if err != nil {
				return data, v, err
			}

			d[string(a)] = v
		} else {
			return nil, nil, errors.New("dictionary key is not a string")
		}

		if len(data) > 0 && data[0] == 0x65 {
			data = data[1:]
			return data, d, nil
		}
	}
	return nil, nil, errors.New("missing e in the end of the dictionary")
}

func decoderHandleValue(data []byte) ([]byte, any, error) {
	if data[0] >= 0x30 && data[0] <= 0x39 {
		return decoderHandleString(data)
	} else if data[0] == 0x69 {
		return decoderHandleInt(data)
	} else if data[0] == 0x6c {
		return decoderHandleList(data)
	} else if data[0] == 0x64 {
		return decoderHandleDictionary(data)
	} else {
		return nil, nil, errors.New("invaild data")
	}
}

func BencodeDecode(data []byte) (int, any, error) {
	if len(data) == 0 {
		return len(data), nil, errors.New("no data to decode")
	} else {
		dataLeft, v, err := decoderHandleValue(data)
		return len(data) - len(dataLeft), v, err
	}
}
