package column

import (
	"github.com/kshvakov/clickhouse/lib/binary"
)

type Int8 struct{ base }

func (Int8) Read(decoder *binary.Decoder) (interface{}, error) {
	v, err := decoder.Int8()
	if err != nil {
		return int8(0), err
	}
	return v, nil
}

func (Int8) Write(encoder *binary.Encoder, v interface{}) error {
	switch v := v.(type) {
	case int8:
		return encoder.Int8(v)
	case int64:
		return encoder.Int8(int8(v))
	}
	return &ErrUnexpectedType{v}
}