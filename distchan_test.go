package distchan_test

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"io"
	"io/ioutil"
	"net"
	"reflect"
	"testing"

	"github.com/covrom/distchan"
)

func TestDistchan(t *testing.T) {
	ln, err := net.Listen("tcp", "localhost:9450")
	if err != nil {
		t.Fatal(err)
	}

	sch := make(chan string)
	server, _ := distchan.NewServer(ln, sch, nil)
	server.Start()

	conn, err := net.Dial(ln.Addr().Network(), ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	inCh := make(chan string)
	cli, _ := distchan.NewClient(conn, nil, inCh)
	cli.Start()

	go func() {
		sch <- "why"
		sch <- "hello"
		sch <- "there"
		sch <- "world"
		close(sch)
	}()

	var received []string
	for msg := range inCh {
		received = append(received, msg)
	}

	if !reflect.DeepEqual(received, []string{"why", "hello", "there", "world"}) {
		t.Errorf("received unexpected values: %v", received)
	}
}

func Encrypter(key []byte) distchan.Transformer {
	return func(buf io.Reader) io.Reader {
		block, err := aes.NewCipher(key)
		if err != nil {
			panic(err)
		}
		plaintext, _ := ioutil.ReadAll(buf)
		ciphertext := make([]byte, aes.BlockSize+len(plaintext))
		iv := ciphertext[:aes.BlockSize]
		if _, err := io.ReadFull(rand.Reader, iv); err != nil {
			panic(err)
		}

		stream := cipher.NewCFBEncrypter(block, iv)
		stream.XORKeyStream(ciphertext[aes.BlockSize:], plaintext)

		return bytes.NewReader(ciphertext)
	}
}

func Decrypter(key []byte) distchan.Transformer {
	return func(buf io.Reader) io.Reader {
		block, err := aes.NewCipher(key)
		if err != nil {
			panic(err)
		}
		ciphertext,_ := ioutil.ReadAll(buf)
		if len(ciphertext) < aes.BlockSize {
			panic("ciphertext too short")
		}
		iv := ciphertext[:aes.BlockSize]
		ciphertext = ciphertext[aes.BlockSize:]

		stream := cipher.NewCFBDecrypter(block, iv)
		stream.XORKeyStream(ciphertext, ciphertext)

		return bytes.NewReader(ciphertext)
	}
}

func TestEncryptionSuccess(t *testing.T) {
	ln, err := net.Listen("tcp", "localhost:9450")
	if err != nil {
		t.Fatal(err)
	}

	sch := make(chan string)
	server, _ := distchan.NewServer(ln, sch, nil)
	server.AddEncoder(Encrypter([]byte("the-key-has-to-be-32-bytes-long!")))
	server.Start()

	go func() {
		sch <- "why"
		sch <- "hello"
		sch <- "there"
		sch <- "world"
		close(sch)
	}()

	conn, err := net.Dial(ln.Addr().Network(), ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	inCh := make(chan string)
	client, _ := distchan.NewClient(conn, nil, inCh)
	client.AddDecoder(Decrypter([]byte("the-key-has-to-be-32-bytes-long!")))
	client.Start()

	var received []string
	for msg := range inCh {
		received = append(received, msg)
	}

	if !reflect.DeepEqual(received, []string{"why", "hello", "there", "world"}) {
		t.Errorf("received unexpected values: %v", received)
	}
}

func TestEncryptionFailure(t *testing.T) {
	ln, err := net.Listen("tcp", "localhost:9450")
	if err != nil {
		t.Fatal(err)
	}

	sch := make(chan string)
	server, _ := distchan.NewServer(ln, sch, nil)
	server.AddEncoder(Encrypter([]byte("the-key-has-to-be-32-bytes-long!")))
	server.Start()

	go func() {
		sch <- "why"
		sch <- "hello"
		sch <- "there"
		sch <- "world"
		close(sch)
	}()

	conn, err := net.Dial(ln.Addr().Network(), ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	inCh := make(chan string)
	client, _ := distchan.NewClient(conn, nil, inCh)
	client.Logger().SetOutput(ioutil.Discard)
	// Note that the key here is different.
	client.AddDecoder(Decrypter([]byte("the-key-has-to-be-32-bytes-long?")))
	client.Start()

	var received []string
	for msg := range inCh {
		received = append(received, msg)
	}

	if len(received) > 0 {
		t.Error("received values when none were expected")
	}
}
