package util

import (
	"io"

	"github.com/glycerine/rbuf"
	"github.com/orktes/orlang/ast"
)

type HistoryReader struct {
	io.Reader
	column int
	line   int
	buf    *rbuf.FixedSizeRingBuf
}

func NewHistoryReader(in io.Reader) (r *HistoryReader) {
	r = &HistoryReader{
		buf: rbuf.NewFixedSizeRingBuf(1024), // TODO figure out proper size
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
