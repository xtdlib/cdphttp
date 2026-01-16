package test

import (
	"testing"

	"github.com/xtdlib/cdphttp"
)

func TestBasic(t *testing.T) {
	cli := cdphttp.NewClient("http://localhost:9222")
	_ = cli
}
