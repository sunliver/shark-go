package protocol

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"

	"github.com/satori/go.uuid"
)

type BlockData struct {
	ID          uuid.UUID
	Type        byte
	BlockNum    uint32
	BodyCRC32   uint32
	Length      int32
	HeaderCRC32 uint32
	Data        []byte
}

type DisconnectData []string

func (b *BlockData) String() string {
	return fmt.Sprintf("%v:%v:%v:%v", b.ID, b.Type, b.BlockNum, b.Length)
}

// NewGUID returns uuid v4
func NewGUID() uuid.UUID {
	return uuid.NewV4()
}

var ErrBrokenBytes = fmt.Errorf("data: at least need %d bytes", ConstBlockHeaderSzB)
var ErrInvalidBlock = errors.New("data: invalid block")

// Marshal blockdata into bytes
func Marshal(b *BlockData) []byte {
	if b == nil {
		return nil
	}
	b.Length = int32(len(b.Data))
	b.BodyCRC32 = crc32.ChecksumIEEE(b.Data)

	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, b.ID)
	binary.Write(buf, binary.LittleEndian, b.Type)
	binary.Write(buf, binary.LittleEndian, b.BlockNum)
	binary.Write(buf, binary.LittleEndian, b.BodyCRC32)
	binary.Write(buf, binary.LittleEndian, b.Length)
	b.HeaderCRC32 = crc32.ChecksumIEEE(buf.Bytes())
	binary.Write(buf, binary.LittleEndian, b.HeaderCRC32)
	binary.Write(buf, binary.LittleEndian, b.Data)

	return buf.Bytes()
}

// UnMarshal bytes into blockdata
func UnMarshal(b []byte) (blockdata *BlockData, err error) {
	blockdata, err = UnMarshalHeader(b)

	if len(b) > ConstBlockHeaderSzB {
		blockdata.Data = b[ConstBlockHeaderSzB:]

		if blockdata.BodyCRC32 != crc32.ChecksumIEEE(blockdata.Data) {
			blockdata.Type = ConstBlockTypeInvalid
			err = ErrInvalidBlock
		}
	}
	return
}

// UnMarshalHeader bytes into blockdata header; skip check data crc32
func UnMarshalHeader(b []byte) (blockdata *BlockData, err error) {
	if len(b) < ConstBlockHeaderSzB {
		return nil, ErrBrokenBytes
	}

	blockdata = &BlockData{}
	blockdata.ID, _ = uuid.FromBytes(b[:16])
	buf := bytes.NewBuffer(b[16:])
	binary.Read(buf, binary.LittleEndian, &blockdata.Type)
	binary.Read(buf, binary.LittleEndian, &blockdata.BlockNum)
	binary.Read(buf, binary.LittleEndian, &blockdata.BodyCRC32)
	binary.Read(buf, binary.LittleEndian, &blockdata.Length)
	binary.Read(buf, binary.LittleEndian, &blockdata.HeaderCRC32)

	if blockdata.HeaderCRC32 != crc32.ChecksumIEEE(b[:ConstBlockHeaderSzB-4]) {
		blockdata.Type = ConstBlockTypeInvalid
		err = ErrInvalidBlock
	}

	return
}
