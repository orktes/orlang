package parser

var keywords = []string{}

var (
	keywordFunction  = registerKeyword("fn")
	keywordReturn    = registerKeyword("return")
	keywordIf        = registerKeyword("if")
	keywordElse      = registerKeyword("else")
	keywordFor       = registerKeyword("for")
	keywordVar       = registerKeyword("var")
	keywordConst     = registerKeyword("const")
	keywordExtern    = registerKeyword("extern")
	keywordMacro     = registerKeyword("macro")
	keywordStruct    = registerKeyword("struct")
	keywordInterface = registerKeyword("interface")
	keywordBreak     = registerKeyword("break")
	keywordContinue  = registerKeyword("continue")
	keywordSwitch    = registerKeyword("switch")
	keywordCase      = registerKeyword("case")
	keywordDefault   = registerKeyword("default")
	keywordDefer     = registerKeyword("defer")
	keywordIn        = registerKeyword("in")
	keywordEnum      = registerKeyword("enum")
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
