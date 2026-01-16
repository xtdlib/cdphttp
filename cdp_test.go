package cdphttp

import (
	"context"
	"fmt"
	"io"
	"log"
	"testing"
)

func must1[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

func TestCookie(t *testing.T) {
	cli := NewClient("http://oci-aca-001:9222")

	{
		resp := must1(cli.Get("https://httpbin.org/cookies/set/y/b"))
		defer resp.Body.Close()
		body := must1(io.ReadAll(resp.Body))
		log.Printf("Response Body: %s", body)
	}

	{
		resp := must1(cli.Get("https://httpbin.org/cookies"))
		defer resp.Body.Close()
		body := must1(io.ReadAll(resp.Body))
		log.Printf("Response Body: %s", body)
	}
}

func TestDebug(t *testing.T) {
	ctx := context.Background()

	cdpcllient, err := createCDPClient(ctx, "ws://localhost:9222")
	_ = cdpcllient
	if err != nil {
		t.Fatal(err)
	}

	result, err := cdpcllient.execute(ctx, "Storage.getCookies", nil)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(string(result))
}
