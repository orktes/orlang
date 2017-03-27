package parser

var keywords = []string{}

var (
	KeywordFunction = registerKeyword("fn")
	KeywordReturn   = registerKeyword("return")
	KeywordIf       = registerKeyword("if")
	KeywordElse     = registerKeyword("else")
	KeywordFor      = registerKeyword("for")
	KeywordVar      = registerKeyword("var")
	KeywordConst    = registerKeyword("const")
	KeywordExtern   = registerKeyword("extern")
	KeywordMacro    = registerKeyword("macro")
)

func registerKeyword(kw string) string {
	keywords = append(keywords, kw)
	return kw
}

func isKeyword(kw string) bool {
	for _, keyword := range keywords {
		if keyword == kw {
			return true
		}
	}

	return false
}
