package helper

import (
	"fmt"
	"io"
	"net"
	"testing"

	uuid "github.com/satori/go.uuid"
	"github.com/sunliver/shark-go/protocol"
	"github.com/sunliver/shark-go/utils"
)

func NewEchoServer(t *testing.T, port int, uuid uuid.UUID) {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		t.Fatal(err)
	}
	defer func(t *testing.T) {
		listener.Close()
	}(t)

	// for
	{
		conn, err := listener.Accept()
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()

		// handshake 1
		{
			header := make([]byte, protocol.ConstBlockHeaderSzB)
			if n, err := io.ReadFull(conn, header); err != nil || n != protocol.ConstBlockHeaderSzB {
				t.Fatal(err)
			}

			blockdata, err := protocol.UnMarshal(header)
			if err != nil {
				t.Fatal(err)
			}

			if blockdata.Type != protocol.ConstBlockTypeHandShake {
				t.Fatal("invalid handshake")
			}

			if _, err := conn.Write(protocol.Marshal(&protocol.BlockData{
				ID:   uuid,
				Type: protocol.ConstBlockTypeHandShake,
			})); err != nil {
				t.Fatal(err)
			}
		}

		// handshake 2
		var passwd []byte
		{
			header := make([]byte, protocol.ConstBlockHeaderSzB)

			if n, err := io.ReadFull(conn, header); err != nil || n != protocol.ConstBlockHeaderSzB {
				t.Fatal(err)
			}

			blockHeader, err := protocol.UnMarshalHeader(header)
			if err != nil {
				t.Fatal(err)
			}

			if blockHeader.Type != protocol.ConstBlockTypeHandShakeResponse {
				t.Fatal("invalid handshake resp")
			}

			body := make([]byte, blockHeader.Length)
			if n, err := io.ReadFull(conn, body); err != nil || int32(n) != blockHeader.Length {
				t.Fatal(err)
			}

			passwd = body

			if _, err := conn.Write(protocol.Marshal(&protocol.BlockData{
				ID:   uuid,
				Type: protocol.ConstBlockTypeHandShakeFinal,
			})); err != nil {
				t.Fatal(err)
			}
		}

		crypto := utils.NewCrypto(passwd)

		for {
			header := make([]byte, protocol.ConstBlockHeaderSzB)
			if n, err := io.ReadFull(conn, header); err != nil || n != protocol.ConstBlockHeaderSzB {
				t.Fatal(err)
			}

			blockHeader, err := protocol.UnMarshal(header)
			if err != nil {
				t.Fatal(err)
			}

			switch blockHeader.Type {
			case protocol.ConstBlockTypeConnect:
				body := make([]byte, blockHeader.Length)
				if n, err := io.ReadFull(conn, body); err != nil || n != int(blockHeader.Length) {
					t.Fatal(err)
				}
				t.Logf("remote server recv connect msg, %v", string(crypto.DecrypBlocks(body)))

				if _, err := conn.Write(protocol.Marshal(&protocol.BlockData{
					ID:   blockHeader.ID,
					Type: protocol.ConstBlockTypeConnected,
				})); err != nil {
					t.Fatal(err)
				}
			case protocol.ConstBlockTypeData:
				body := make([]byte, blockHeader.Length)
				if n, err := io.ReadFull(conn, body); err != nil || int32(n) != blockHeader.Length {
					t.Fatal(err)
				}

				// t.Logf("recv data, %v", string(crypto.DecrypBlocks(body)))

				if _, err := conn.Write(protocol.Marshal(&protocol.BlockData{
					ID:   blockHeader.ID,
					Type: protocol.ConstBlockTypeData,
					Data: body,
				})); err != nil {
					t.Fatal(err)
				}

			case protocol.ConstBlockTypeDisconnect:
				t.Log("receive disconnect")
				return
			}
		}
	}
}
