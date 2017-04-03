package parser

import (
	"github.com/orktes/orlang/parser/ast"
	"github.com/orktes/orlang/parser/scanner"
)

func (p *Parser) parseBlock() (node *ast.Block, ok bool) {
	if node, ok = p.parseMacroSubstitutionBlock(); ok {
		return
	}

	if _, lok := p.expectToken(scanner.TokenTypeLBRACE); !lok {
		p.unread()
		return
	}

	node = &ast.Block{}

loop:
	for {
		var blockNode ast.Node
		var check = func(n ast.Node, ok bool) bool {
			if ok {
				blockNode = n
			}
			return ok
		}
		switch {
		case check(p.parseStatementOrExpression(true)):
		default:
			if _, tok := p.expectToken(scanner.TokenTypeRBRACE); !tok {
				p.unread()
				return
			}
			break loop
		}

		if blockNode != nil {
			node.AppendNode(blockNode)
		}
	}

	ok = true

	return
}
