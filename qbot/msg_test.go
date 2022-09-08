package qbot

import (
	"github.com/stretchr/testify/assert"
	"math/rand"
	"testing"
	"time"
)

func TestUnescapeCQCode(t *testing.T) {
	tests := []struct {
		name string
		in   string
		out  string
	}{
		{"UnEscape &amp; in start", "&amp; test case", "& test case"},
		{"UnEscape &amp; in mid", "test &amp; case", "test & case"},
		{"UnEscape &amp; in end", "test case &amp;", "test case &"},
		{"UnEscape char follow &amp;", "test &amp;case", "test &case"},
		{"UnEscape &amp; follow char", "test&amp; case", "test& case"},
		{"UnEscape &amp; around with char", "test&amp;case", "test&case"},
		{"Unescape multiple &amp;", "&amp; test case &amp; case &amp; case &amp;",
			"& test case & case & case &"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.out, unescapeCQCode(test.in))
		})
	}
}

func TestEscapeCQCode(t *testing.T) {
	tests := []struct {
		name string
		in   string
		out  string
	}{
		{"case & in start", "& test case", "&amp; test case"},
		{"case & in mid", "test & case", "test &amp; case"},
		{"case & in end", "test case &", "test case &amp;"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.out, escapeCQCode(test.in))
		})
	}
}

// 使用随机字符串进行测试
func TestEscapeUnEscapeCQCode(t *testing.T) {
	chars := []byte{'&', '[', ']', ','}
	rand.Seed(time.Now().UnixNano())
	t.Logf("test escape and unescape with random string")
	randStr := func() string {
		size := rand.Intn(100)
		temp := make([]byte, size)
		for i := range temp {
			cn := rand.Intn(26 + len(chars) + 1)
			var c byte
			if cn > 26 {
				c = chars[cn-26-1]
			} else {
				c = byte(cn) + 'a'
			}
			temp[i] = c
		}
		return string(temp)
	}
	for i := 0; i < 100; i++ {
		testCase := randStr()
		escape := escapeCQCode(testCase)
		unEscape := unescapeCQCode(escape)
		if unEscape != testCase {
			t.Errorf("randStr: %s, escape: %s, unEscape: %s", testCase, escape, unEscape)
		}
		t.Logf("randStr: %s, escape: %s, unEscape: %s", testCase, escape, unEscape)
	}
}

func TestParseCQCode(t *testing.T) {
	tests := []struct {
		name string
		in   string
		out  *CQCode
	}{
		{"test case at CQCode", "[CQ:at,qq=114514]", &CQCode{
			"at",
			map[string]string{
				"qq": "114514",
			},
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := parseCQCode(test.in)
			assert.Equal(t, test.out.Types, got.Types)
			assert.Equal(t, test.out.Data, got.Data)
		})
	}
}

func TestCQCode_String(t *testing.T) {
	code := &CQCode{
		Types: "at",
		Data: map[string]string{
			"qq": "114514",
		},
	}
	t.Log(code.String())

	reply := &CQCode{
		Types: "reply",
		Data: map[string]string{
			"id": "id",
		},
	}
	t.Logf(reply.String())
}
