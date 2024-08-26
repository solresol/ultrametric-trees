package processor

var punctuationAndPronouns = map[string]bool{
	"i": true, "me": true, "my": true, "mine": true,
	"you": true, "your": true, "u": true,
	"he": true, "him": true, "his": true,
	"she": true, "her": true,
	"it": true, "its": true,
	"we": true, "us": true, "our": true,
	"they": true, "them": true, "their": true,
	"!": true, ".": true, "?": true,
}

func isPunctuationOrPronoun(word string) bool {
	_, ok := punctuationAndPronouns[word]
	return ok
}
