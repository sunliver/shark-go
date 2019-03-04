package helper

import (
	"fmt"
	"io"
	"net"
	"testing"

	uuid "github.com/satori/go.uuid"
	"github.com/sunliver/shark/lib/block"
	"github.com/sunliver/shark/lib/crypto"
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
			header := make([]byte, block.ConstBlockHeaderSzB)
			if n, err := io.ReadFull(conn, header); err != nil || n != block.ConstBlockHeaderSzB {
				t.Fatal(err)
			}

			blockdata, err := block.UnMarshal(header)
			if err != nil {
				t.Fatal(err)
			}

			if blockdata.Type != block.ConstBlockTypeHandShake {
				t.Fatal("invalid handshake")
			}

			if _, err := conn.Write(block.Marshal(&block.BlockData{
				ID:   uuid,
				Type: block.ConstBlockTypeHandShake,
			})); err != nil {
				t.Fatal(err)
			}
		}

		// handshake 2
		var passwd []byte
		{
			header := make([]byte, block.ConstBlockHeaderSzB)

			if n, err := io.ReadFull(conn, header); err != nil || n != block.ConstBlockHeaderSzB {
				t.Fatal(err)
			}

			blockHeader, err := block.UnMarshalHeader(header)
			if err != nil {
				t.Fatal(err)
			}

			if blockHeader.Type != block.ConstBlockTypeHandShakeResponse {
				t.Fatal("invalid handshake resp")
			}

			body := make([]byte, blockHeader.Length)
			if n, err := io.ReadFull(conn, body); err != nil || int32(n) != blockHeader.Length {
				t.Fatal(err)
			}

			passwd = body

			if _, err := conn.Write(block.Marshal(&block.BlockData{
				ID:   uuid,
				Type: block.ConstBlockTypeHandShakeFinal,
			})); err != nil {
				t.Fatal(err)
			}
		}

		crypto := crypto.NewCrypto(passwd)

		for {
			header := make([]byte, block.ConstBlockHeaderSzB)
			if n, err := io.ReadFull(conn, header); err != nil || n != block.ConstBlockHeaderSzB {
				t.Fatal(err)
			}

			blockHeader, err := block.UnMarshal(header)
			if err != nil {
				t.Fatal(err)
			}

			switch blockHeader.Type {
			case block.ConstBlockTypeConnect:
				body := make([]byte, blockHeader.Length)
				if n, err := io.ReadFull(conn, body); err != nil || n != int(blockHeader.Length) {
					t.Fatal(err)
				}
				t.Logf("remote server recv connect msg, %v", string(crypto.DecrypBlocks(body)))

				if _, err := conn.Write(block.Marshal(&block.BlockData{
					ID:   blockHeader.ID,
					Type: block.ConstBlockTypeConnected,
				})); err != nil {
					t.Fatal(err)
				}
			case block.ConstBlockTypeData:
				body := make([]byte, blockHeader.Length)
				if n, err := io.ReadFull(conn, body); err != nil || int32(n) != blockHeader.Length {
					t.Fatal(err)
				}

				// t.Logf("recv data, %v", string(crypto.DecrypBlocks(body)))

				if _, err := conn.Write(block.Marshal(&block.BlockData{
					ID:   blockHeader.ID,
					Type: block.ConstBlockTypeData,
					Data: body,
				})); err != nil {
					t.Fatal(err)
				}

			case block.ConstBlockTypeDisconnect:
				t.Log("receive disconnect")
				return
			}
		}
	}
}
