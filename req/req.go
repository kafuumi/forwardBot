package req

import (
	"fmt"
	"strings"
)

var (
	escapeTable = map[rune]string{
		'\t': `    `,
		'\n': `\n`,
		'\r': `\n`,
		'\b': "",
		'\\': `\\`,
		'"':  `\"`,
		'\f': `\f`,
	}
)

type E struct {
	Name  string
	Value any
}

type D []E

// 处理转义字符
func escapeJson(src string) string {
	cnt := strings.Builder{}
	for _, c := range src {
		if cc, ok := escapeTable[c]; ok {
			cnt.WriteString(cc)
		} else {
			cnt.WriteRune(c)
		}
	}
	return cnt.String()
}

func (e E) Json() string {
	res := strings.Builder{}
	res.WriteString(fmt.Sprintf(`"%s": `, e.Name))
	switch r := e.Value.(type) {
	case E:
		res.WriteString(fmt.Sprintf("{%s}", r.Json()))
	case D:
		res.WriteString(r.Json())
	case fmt.Stringer:
		res.WriteString(escapeJson(fmt.Sprintf(`"%s"`, r.String())))
	case string:
		res.WriteByte('"')
		res.WriteString(escapeJson(r))
		res.WriteByte('"')
	case nil:
		res.WriteString("null")
	default:
		res.WriteString(escapeJson(fmt.Sprint(e.Value)))
	}
	return res.String()
}

func (d D) Json() string {
	res := strings.Builder{}
	res.WriteByte('{')
	for i := range d {
		res.WriteString(d[i].Json())
		if i != len(d)-1 {
			res.WriteByte(',')
		}
	}
	res.WriteByte('}')
	return res.String()
}
