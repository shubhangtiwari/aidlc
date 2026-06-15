package model

import (
	"strings"
	"unicode"
)

var queryStopWords = map[string]struct{}{
	"a": {}, "about": {}, "all": {}, "also": {}, "am": {}, "an": {}, "and": {}, "any": {}, "are": {}, "as": {}, "at": {},
	"be": {}, "been": {}, "by": {},
	"can": {}, "could": {},
	"do": {}, "does": {},
	"for": {}, "from": {},
	"has": {}, "have": {}, "how": {},
	"i": {}, "in": {}, "into": {}, "is": {}, "it": {}, "its": {},
	"me": {},
	"of": {}, "on": {}, "or": {}, "please": {},
	"that": {}, "the": {}, "their": {}, "there": {}, "this": {}, "to": {},
	"was": {}, "we": {}, "what": {}, "when": {}, "where": {}, "which": {}, "who": {}, "why": {}, "with": {},
}

// QueryTerms returns significant lexical search terms in deterministic input order.
func QueryTerms(query string) []string {
	fields := strings.FieldsFunc(strings.ToLower(query), queryTermBoundary)
	terms := make([]string, 0, len(fields)*2)
	seen := make(map[string]struct{}, len(fields)*2)
	for _, field := range fields {
		term := strings.Trim(field, "_-.")
		if term == "" || isQueryStopWord(term) {
			continue
		}
		appendSearchTerm(&terms, seen, term)
	}
	if len(terms) == 0 {
		return nil
	}
	return terms
}

// SearchText returns deterministic search text with code identifiers expanded into natural terms.
func SearchText(parts ...string) string {
	fields := make([]string, 0, len(parts)*4)
	seen := make(map[string]struct{}, len(parts)*4)
	for _, part := range parts {
		if part = strings.TrimSpace(part); part == "" {
			continue
		}
		appendRawSearchText(&fields, seen, part)
		for _, token := range searchTokens(part) {
			appendSearchTerm(&fields, seen, token)
			for _, segment := range splitCodeIdentifier(token) {
				appendSearchTerm(&fields, seen, segment)
				for _, variant := range lexicalVariants(segment) {
					appendSearchTerm(&fields, seen, variant)
				}
			}
			for _, variant := range lexicalVariants(token) {
				appendSearchTerm(&fields, seen, variant)
			}
		}
	}
	return strings.Join(fields, " ")
}

func queryTermBoundary(r rune) bool {
	return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '-' && r != '.' && r != '/'
}

func isQueryStopWord(term string) bool {
	_, ok := queryStopWords[term]
	return ok
}

func appendSearchTerm(terms *[]string, seen map[string]struct{}, term string) {
	term = strings.Trim(strings.ToLower(term), "_-.")
	if term == "" || isQueryStopWord(term) {
		return
	}
	if _, ok := seen[term]; ok {
		return
	}
	seen[term] = struct{}{}
	*terms = append(*terms, term)
}

func appendRawSearchText(terms *[]string, seen map[string]struct{}, term string) {
	key := strings.Trim(strings.ToLower(term), "_-.")
	if key == "" || isQueryStopWord(key) {
		return
	}
	if _, ok := seen[key]; ok {
		return
	}
	seen[key] = struct{}{}
	*terms = append(*terms, term)
}

func searchTokens(text string) []string {
	fields := strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	tokens := make([]string, 0, len(fields))
	for _, field := range fields {
		if token := strings.Trim(field, "_-."); token != "" {
			tokens = append(tokens, token)
		}
	}
	return tokens
}

func splitCodeIdentifier(token string) []string {
	if token == "" {
		return nil
	}
	var segments []string
	var current []rune
	runes := []rune(token)
	for i, r := range runes {
		if len(current) > 0 && isIdentifierBoundary(runes, i) {
			segments = append(segments, strings.ToLower(string(current)))
			current = current[:0]
		}
		current = append(current, r)
	}
	if len(current) > 0 {
		segments = append(segments, strings.ToLower(string(current)))
	}
	return segments
}

func isIdentifierBoundary(runes []rune, i int) bool {
	if i == 0 {
		return false
	}
	prev := runes[i-1]
	curr := runes[i]
	if unicode.IsDigit(prev) != unicode.IsDigit(curr) {
		return true
	}
	if unicode.IsLower(prev) && unicode.IsUpper(curr) {
		return true
	}
	if unicode.IsUpper(prev) && unicode.IsUpper(curr) && i+1 < len(runes) && unicode.IsLower(runes[i+1]) {
		return true
	}
	return false
}

func lexicalVariants(term string) []string {
	var variants []string
	switch {
	case strings.HasSuffix(term, "s") && len(term) > 3:
		variants = append(variants, strings.TrimSuffix(term, "s"))
	case len(term) > 2:
		variants = append(variants, term+"s")
	}
	if term == "extract" {
		variants = append(variants, "extraction")
	}
	if term == "extraction" {
		variants = append(variants, "extract")
	}
	if strings.HasSuffix(term, "ate") && len(term) > 5 {
		variants = append(variants, strings.TrimSuffix(term, "e")+"ion")
	}
	if strings.HasSuffix(term, "ize") && len(term) > 5 {
		variants = append(variants, strings.TrimSuffix(term, "ize")+"ization")
	}
	return variants
}
