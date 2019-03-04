package block

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMarshal(t *testing.T) {

	cases := []BlockData{
		{
			ID:       NewGUID(),
			Type:     ConstBlockTypeData,
			BlockNum: 0x12345678,
			Data:     []byte{0x12, 0x34, 0x56, 0x78},
		},
		{
			ID:       NewGUID(),
			Type:     ConstBlockTypeHandShake,
			BlockNum: 0x12345678,
		},
	}

	for _, b := range cases {
		mb := Marshal(&b)
		ub, err := UnMarshal(mb)

		assert.Nil(t, err)
		assert.Equal(t, b.ID, ub.ID)
		assert.Equal(t, b.Type, ub.Type)
		assert.Equal(t, b.BlockNum, ub.BlockNum)
	}
}

func TestMarshalNil(t *testing.T) {
	mb := Marshal(nil)
	assert.Nil(t, mb)
}

func TestUnMarshalInvalid(t *testing.T) {
	b := BlockData{
		ID:       NewGUID(),
		Type:     ConstBlockTypeData,
		BlockNum: 0x12345678,
		Data:     []byte{0x12, 0x34, 0x56, 0x78},
	}

	mb := Marshal(&b)
	mb[16] = 0xff
	ub, err := UnMarshal(mb)
	assert.Equal(t, ErrInvalidBlock, err)
	assert.Equal(t, ConstBlockTypeInvalid, ub.Type)
}

func TestUnMarshalBroken(t *testing.T) {
	b := BlockData{
		ID:       NewGUID(),
		Type:     ConstBlockTypeData,
		BlockNum: 0x12345678,
		Data:     []byte{0x12, 0x34, 0x56, 0x78},
	}
	mb := Marshal(&b)
	_, err := UnMarshal(mb[:29])
	assert.Equal(t, ErrBrokenBytes, err)
}
