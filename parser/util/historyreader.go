package util

import (
	"fmt"
	"io"
	"strings"

	"github.com/glycerine/rbuf"
	"github.com/orktes/orlang/parser/ast"
	"github.com/orktes/orlang/parser/parser"
)

type HistoryReader struct {
	io.Reader
	column int
	line   int
	buf    *rbuf.FixedSizeRingBuf
}

func NewHistoryReader(in io.Reader) (r *HistoryReader) {
	r = &HistoryReader{
		buf: rbuf.NewFixedSizeRingBuf(1024 * 5), // TODO figure out proper size
	}
	r.Reader = io.TeeReader(in, r)
	return
}

func (hr *HistoryReader) Write(p []byte) (n int, err error) {
	for _, b := range p {
		hr.column++
		if b == byte('\n') {
			hr.line++
			hr.column = 0
		}
	}

	return hr.buf.Write(p)
}

func (hr *HistoryReader) FindLineForPosition(pos ast.Position) string {
	line := hr.line
	bytes := hr.buf.Bytes()
	correctLine := false
	correctLineEndIndex := 0

	for i := len(bytes) - 1; i >= 0; i-- {
		b := bytes[i]
		if correctLine == false && line == pos.Line {
			correctLineEndIndex = i + 1
			correctLine = true
		}
		if b == '\n' || i == 0 {
			if correctLine {
				if i != 0 {
					i++
				}
				return string(bytes[i:correctLineEndIndex])
			}
			line--
		}
	}

	return ""
}

func (hr *HistoryReader) FormatParseError(filePath string, pos ast.Position, err string) string {
	line := hr.FindLineForPosition(pos)
	if line != "" {
		return fmt.Sprintf(
			`%s:%d:%d
----------------------------------------------------------
%s
%s
%s
----------------------------------------------------------`, filePath, pos.Line+1, pos.Column+1, strings.Replace(line, "\t", " ", -1), pointer(pos.Column), pad(pos.Column-int(len(err)/2), err))
	}
	return fmt.Sprintf("%s %#v %s", filePath, pos, err)
}

func (hr *HistoryReader) FormatError(filePath string, err error) string {
	if posErr, ok := err.(*parser.PosError); ok && hr != nil {
		return hr.FormatParseError(filePath, posErr.Position, posErr.Message)
	}
	return fmt.Sprintf("%s:%s", filePath, err.Error())
}

func pad(padding int, str string) (res string) {
	if padding < 0 {
		padding = 0
	}
	res = strings.Repeat(" ", padding)
	return res + str
}

func pointer(padding int) (res string) {
	return pad(padding, "^")
}
