package localclient

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/sunliver/shark-go/client/helper"
	"github.com/sunliver/shark-go/protocol"
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

	client.RegisterObserver(uuid, func(data *protocol.BlockData, err error) {
		if err == nil {
			t.Logf("receive msg: %v", string(data.Data))
		}
	})

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

	client.UnRegisterObserver(uuid)
	client.release()
}
