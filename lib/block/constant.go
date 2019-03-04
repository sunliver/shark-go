package block

// block types
const (
	ConstBlockTypeHandShake         = byte(0x00)
	ConstBlockTypeHandShakeResponse = byte(0x01)
	ConstBlockTypeHandShakeFinal    = byte(0x02)
	ConstBlockTypeConnect           = byte(0x03)
	ConstBlockTypeConnected         = byte(0x04)
	ConstBlockTypeRequestResend     = byte(0x05)
	ConstBlockTypeData              = byte(0x06)
	ConstBlockTypeDisconnect        = byte(0x07)
	ConstBlockTypeFastConnect       = byte(0xA0)
	ConstBlockTypeConnectFailed     = byte(0xF0)
	ConstBlockTypeInvalid           = byte(0xFF)
)

// block header size
const (
	ConstBlockHeaderSzB = 33
)
