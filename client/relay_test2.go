package client

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/sunliver/shark/client/helper"
	"github.com/sunliver/shark/protocol"
)

func TestClientOK(t *testing.T) {
	uuid := protocol.NewGUID()

	go helper.NewEchoServer(t, 9001, uuid)

	conf := RemoteProxyConf{
		RemoteServer: "localhost",
		RemotePort:   9001,
	}

	client, err := GetSingleClient(conf)
	if !assert.Nil(t, err) ||
		!assert.NotNil(t, client) ||
		!assert.Equal(t, constStatusRunning, client.Status) {
		t.FailNow()
	}

	client.Write(&protocol.BlockData{
		ID:   uuid,
		Type: protocol.ConstBlockTypeData,
		Data: []byte("Hello"),
	})

	client.Write(&protocol.BlockData{
		ID:   uuid,
		Type: protocol.ConstBlockTypeDisconnect,
	})

	time.Sleep(time.Millisecond * 100)

	client.release()
}
