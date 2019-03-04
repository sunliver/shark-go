package client

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/sunliver/shark/lib/block"
	"github.com/sunliver/shark/test/helper"
)

func TestClientOK(t *testing.T) {
	uuid := block.NewGUID()

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

	client.Write(&block.BlockData{
		ID:   uuid,
		Type: block.ConstBlockTypeData,
		Data: []byte("Hello"),
	})

	client.Write(&block.BlockData{
		ID:   uuid,
		Type: block.ConstBlockTypeDisconnect,
	})

	time.Sleep(time.Millisecond * 100)

	client.release()
}
