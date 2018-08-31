package main

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	coap "github.com/OSSystems/go-coap"
	"github.com/boltdb/bolt"
	"github.com/gustavosbarreto/cdn/journal"
	"github.com/gustavosbarreto/cdn/objstore"
	"github.com/gustavosbarreto/cdn/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCoapHandler(t *testing.T) {
	dir, err := ioutil.TempDir("", "test")
	assert.NoError(t, err)

	defer os.RemoveAll(dir)

	db, err := bolt.Open(filepath.Join(dir, "db"), 0600, nil)
	assert.NoError(t, err)

	mm := &mockMonitor{}

	app := &App{
		journal: journal.NewJournal(db, -1),
		storage: storage.NewStorage(dir),
		monitor: mm,
	}

	data := make([]byte, 4)
	rand.Read(data)

	sv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "file", time.Now(), bytes.NewReader(data))
	}))

	sv.Start()
	defer sv.Close()

	mm.On("RecordMetric", "file", mock.Anything, int64(len(data)), int64(len(data)), mock.Anything).Return()

	app.objstore = objstore.NewObjStore(fmt.Sprintf("http://%s", sv.Listener.Addr().String()), app.journal, app.storage)

	uaddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1")

	msg := &coap.Message{
		Code:   coap.GET,
		Block2: &coap.Block{Size: uint32(len(data))},
	}

	msg.SetPathString("/file")

	res := app.ServeCOAP(nil, uaddr, msg)
	assert.NotNil(t, res)
	assert.Equal(t, coap.Content, res.Code)
	assert.Equal(t, data, res.Payload)

	mm.AssertExpectations(t)
}
