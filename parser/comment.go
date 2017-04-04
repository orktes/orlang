package parser

import (
	"github.com/orktes/orlang/ast"
	"github.com/orktes/orlang/scanner"
)

func (p *Parser) processComment(tok scanner.Token) {
	comment := ast.Comment{Token: tok}
	if p.commentAfterNodeCheck != nil {
		node := p.commentAfterNodeCheck
		if comment.Token.EndLine == node.EndPos().Line {
			p.nodeComments[node] = append(p.nodeComments[node], comment)
			return
		}
	}
	p.comments = append(p.comments, comment)
}

func (p *Parser) checkCommentForNode(node ast.Node, afterNode bool) {
	if len(p.comments) == 0 {
		return
	}

	for i := len(p.comments) - 1; i >= 0; i-- {
		lastComment := p.comments[i]
		diff := node.StartPos().Line - lastComment.Token.EndLine

		if diff == 1 || diff == 0 {
			p.comments = append(p.comments[:i], p.comments[i+1:]...)
			p.nodeComments[node] = append(p.nodeComments[node], lastComment)
		} else if diff > 1 {
			break
		}
	}

	if afterNode {
		p.commentAfterNodeCheck = node
	}

	return
}
