package migration

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

var ErrSQLUnparseable = errors.New("PACKAGE_MIGRATION_SQL_UNPARSEABLE")

type SQLStatementType string

const (
	SQLTypeCreateTable   SQLStatementType = "CREATE_TABLE"
	SQLTypeCreateView    SQLStatementType = "CREATE_VIEW"
	SQLTypeCreateTrigger SQLStatementType = "CREATE_TRIGGER"
	SQLTypeCreateIndex   SQLStatementType = "CREATE_INDEX"
	SQLTypeAlterTable    SQLStatementType = "ALTER_TABLE"
	SQLTypeDropTable     SQLStatementType = "DROP_TABLE"
	SQLTypeDropView      SQLStatementType = "DROP_VIEW"
	SQLTypeDropTrigger   SQLStatementType = "DROP_TRIGGER"
	SQLTypeDropIndex     SQLStatementType = "DROP_INDEX"
	SQLTypeInsert        SQLStatementType = "INSERT"
	SQLTypeUpdate        SQLStatementType = "UPDATE"
	SQLTypeDelete        SQLStatementType = "DELETE"
	SQLTypeSelect        SQLStatementType = "SELECT"
	SQLTypeOther         SQLStatementType = "OTHER"
)

type SQLObjectRef struct {
	Name string
	Kind string
}

type SQLStatement struct {
	Raw     string
	Type    SQLStatementType
	Objects []SQLObjectRef
}

func (s *SQLStatement) AllObjectNames() []string {
	names := make([]string, 0, len(s.Objects))
	for _, obj := range s.Objects {
		names = append(names, obj.Name)
	}
	return names
}

type sqlTokenKind int

const (
	tokWord sqlTokenKind = iota
	tokIdent
	tokString
	tokNumber
	tokPunct
	tokEOF
)

type sqlToken struct {
	kind  sqlTokenKind
	value string
}

func isSQLIdentChar(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

func isSQLIdentStart(r rune) bool {
	return r == '_' || unicode.IsLetter(r)
}

func tokenizeSQL(input string) []sqlToken {
	var tokens []sqlToken
	runes := []rune(input)
	n := len(runes)
	i := 0
	for i < n {
		r := runes[i]
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			i++
			continue
		}
		if r == '-' && i+1 < n && runes[i+1] == '-' {
			for i < n && runes[i] != '\n' {
				i++
			}
			continue
		}
		if r == '/' && i+1 < n && runes[i+1] == '*' {
			i += 2
			for i+1 < n && !(runes[i] == '*' && runes[i+1] == '/') {
				i++
			}
			if i+1 < n {
				i += 2
			} else {
				i = n
			}
			continue
		}
		if r == '\'' {
			start := i
			i++
			for i < n {
				if runes[i] == '\'' {
					if i+1 < n && runes[i+1] == '\'' {
						i += 2
						continue
					}
					i++
					break
				}
				i++
			}
			tokens = append(tokens, sqlToken{kind: tokString, value: string(runes[start:i])})
			continue
		}
		if r == '"' {
			start := i
			i++
			for i < n {
				if runes[i] == '"' {
					if i+1 < n && runes[i+1] == '"' {
						i += 2
						continue
					}
					i++
					break
				}
				i++
			}
			inner := string(runes[start+1 : i-1])
			inner = strings.ReplaceAll(inner, `""`, `"`)
			tokens = append(tokens, sqlToken{kind: tokIdent, value: inner})
			continue
		}
		if r == '`' {
			start := i
			i++
			for i < n && runes[i] != '`' {
				i++
			}
			if i < n {
				i++
			}
			inner := string(runes[start+1 : i-1])
			tokens = append(tokens, sqlToken{kind: tokIdent, value: inner})
			continue
		}
		if r == '[' {
			start := i
			i++
			for i < n && runes[i] != ']' {
				i++
			}
			if i < n {
				i++
			}
			inner := string(runes[start+1 : i-1])
			tokens = append(tokens, sqlToken{kind: tokIdent, value: inner})
			continue
		}
		if unicode.IsDigit(r) {
			start := i
			for i < n && (unicode.IsDigit(runes[i]) || runes[i] == '.' || runes[i] == 'e' || runes[i] == 'E') {
				i++
			}
			tokens = append(tokens, sqlToken{kind: tokNumber, value: string(runes[start:i])})
			continue
		}
		if isSQLIdentStart(r) {
			start := i
			for i < n && isSQLIdentChar(runes[i]) {
				i++
			}
			tokens = append(tokens, sqlToken{kind: tokWord, value: string(runes[start:i])})
			continue
		}
		tokens = append(tokens, sqlToken{kind: tokPunct, value: string(r)})
		i++
	}
	tokens = append(tokens, sqlToken{kind: tokEOF, value: ""})
	return tokens
}

type sqlTokenStream struct {
	tokens []sqlToken
	pos    int
}

func newTokenStream(tokens []sqlToken) *sqlTokenStream {
	return &sqlTokenStream{tokens: tokens, pos: 0}
}

func (s *sqlTokenStream) peek() sqlToken {
	if s.pos < len(s.tokens) {
		return s.tokens[s.pos]
	}
	return sqlToken{kind: tokEOF, value: ""}
}

func (s *sqlTokenStream) next() sqlToken {
	tok := s.peek()
	if s.pos < len(s.tokens) {
		s.pos++
	}
	return tok
}

func (s *sqlTokenStream) peekWord() string {
	tok := s.peek()
	if tok.kind == tokWord {
		return strings.ToUpper(tok.value)
	}
	return ""
}

func (s *sqlTokenStream) consumeWord(expected string) bool {
	tok := s.peek()
	if tok.kind == tokWord && strings.ToUpper(tok.value) == expected {
		s.pos++
		return true
	}
	return false
}

func (s *sqlTokenStream) consumePunct(expected string) bool {
	tok := s.peek()
	if tok.kind == tokPunct && tok.value == expected {
		s.pos++
		return true
	}
	return false
}

func (s *sqlTokenStream) readObjectName() (string, bool) {
	tok := s.peek()
	if tok.kind == tokIdent {
		s.pos++
		return tok.value, true
	}
	if tok.kind == tokWord {
		s.pos++
		return tok.value, true
	}
	return "", false
}

func (s *sqlTokenStream) skipIFClause() {
	s.consumeWord("IF")
	if s.peekWord() == "NOT" {
		s.next()
	}
	if s.peekWord() == "EXISTS" {
		s.next()
	} else {
		s.consumeWord("EXISTS")
	}
}

func isReservedWord(word string) bool {
	switch strings.ToUpper(word) {
	case "IF", "NOT", "EXISTS", "TEMP", "TEMPORARY", "UNIQUE", "OR",
		"REPLACE", "ROLLBACK", "ABORT", "FAIL", "IGNORE", "BEFORE", "AFTER",
		"INSTEAD", "OF", "FOR", "EACH", "ROW", "WHEN", "BEGIN", "END",
		"ON", "AS", "SET", "WHERE", "VALUES", "SELECT", "FROM", "JOIN",
		"INNER", "LEFT", "RIGHT", "FULL", "CROSS", "NATURAL", "OUTER",
		"GROUP", "ORDER", "HAVING", "LIMIT", "OFFSET", "AND", "INTO",
		"TABLE", "VIEW", "TRIGGER", "INDEX", "COLUMN", "RENAME", "TO",
		"ADD", "DROP", "ALTER", "CREATE", "INSERT", "UPDATE", "DELETE",
		"CONSTRAINT", "PRIMARY", "KEY", "FOREIGN", "REFERENCES", "DEFAULT",
		"NULL", "CHECK", "COLLATE", "WITHOUT", "ROWID", "USING":
		return true
	}
	return false
}

func ParseStatement(raw string) (*SQLStatement, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, ErrSQLUnparseable
	}
	tokens := tokenizeSQL(raw)
	if len(tokens) == 0 {
		return nil, ErrSQLUnparseable
	}
	stream := newTokenStream(tokens)
	first := stream.peek()
	if first.kind != tokWord {
		return nil, ErrSQLUnparseable
	}
	upperFirst := strings.ToUpper(first.value)
	switch upperFirst {
	case "CREATE":
		return parseCreateStatement(stream, raw)
	case "ALTER":
		return parseAlterStatement(stream, raw)
	case "DROP":
		return parseDropStatement(stream, raw)
	case "INSERT":
		return parseInsertStatement(stream, raw)
	case "UPDATE":
		return parseUpdateStatement(stream, raw)
	case "DELETE":
		return parseDeleteStatement(stream, raw)
	case "SELECT":
		return parseSelectStatement(stream, raw)
	case "WITH":
		return parseSelectStatement(stream, raw)
	default:
		return &SQLStatement{Raw: raw, Type: SQLTypeOther, Objects: nil}, nil
	}
}

func parseCreateStatement(s *sqlTokenStream, raw string) (*SQLStatement, error) {
	s.next()
	for {
		w := s.peekWord()
		if w == "TEMP" || w == "TEMPORARY" || w == "UNIQUE" {
			s.next()
			continue
		}
		break
	}
	objType := s.peekWord()
	switch objType {
	case "TABLE":
		s.next()
		if s.peekWord() == "IF" {
			s.skipIFClause()
		}
		name, ok := s.readObjectName()
		if !ok {
			return nil, fmt.Errorf("%w: CREATE TABLE missing table name", ErrSQLUnparseable)
		}
		return &SQLStatement{Raw: raw, Type: SQLTypeCreateTable, Objects: []SQLObjectRef{{Name: name, Kind: "table"}}}, nil
	case "VIEW":
		s.next()
		if s.peekWord() == "IF" {
			s.skipIFClause()
		}
		name, ok := s.readObjectName()
		if !ok {
			return nil, fmt.Errorf("%w: CREATE VIEW missing view name", ErrSQLUnparseable)
		}
		refs := []SQLObjectRef{{Name: name, Kind: "view"}}
		extra := extractTableRefsFromStream(s)
		refs = append(refs, extra...)
		return &SQLStatement{Raw: raw, Type: SQLTypeCreateView, Objects: refs}, nil
	case "TRIGGER":
		s.next()
		if s.peekWord() == "IF" {
			s.skipIFClause()
		}
		name, ok := s.readObjectName()
		if !ok {
			return nil, fmt.Errorf("%w: CREATE TRIGGER missing trigger name", ErrSQLUnparseable)
		}
		refs := []SQLObjectRef{{Name: name, Kind: "trigger"}}
		for {
			w := s.peekWord()
			if w == "" || w == "BEGIN" {
				break
			}
			if w == "ON" {
				s.next()
				tblName, tblOk := s.readObjectName()
				if tblOk {
					refs = append(refs, SQLObjectRef{Name: tblName, Kind: "table"})
				}
				break
			}
			s.next()
		}
		bodyRefs := extractTableRefsFromStream(s)
		refs = append(refs, bodyRefs...)
		return &SQLStatement{Raw: raw, Type: SQLTypeCreateTrigger, Objects: refs}, nil
	case "INDEX":
		s.next()
		if s.peekWord() == "IF" {
			s.skipIFClause()
		}
		name, ok := s.readObjectName()
		if !ok {
			return nil, fmt.Errorf("%w: CREATE INDEX missing index name", ErrSQLUnparseable)
		}
		refs := []SQLObjectRef{{Name: name, Kind: "index"}}
		if s.consumeWord("ON") {
			tblName, tblOk := s.readObjectName()
			if tblOk {
				refs = append(refs, SQLObjectRef{Name: tblName, Kind: "table"})
			}
		}
		return &SQLStatement{Raw: raw, Type: SQLTypeCreateIndex, Objects: refs}, nil
	default:
		return nil, fmt.Errorf("%w: unsupported CREATE %s", ErrSQLUnparseable, objType)
	}
}

func parseAlterStatement(s *sqlTokenStream, raw string) (*SQLStatement, error) {
	s.next()
	if !s.consumeWord("TABLE") {
		return nil, fmt.Errorf("%w: ALTER missing TABLE", ErrSQLUnparseable)
	}
	name, ok := s.readObjectName()
	if !ok {
		return nil, fmt.Errorf("%w: ALTER TABLE missing table name", ErrSQLUnparseable)
	}
	refs := []SQLObjectRef{{Name: name, Kind: "table"}}
	action := s.peekWord()
	if action == "RENAME" {
		s.next()
		if s.peekWord() == "TO" {
			s.next()
			newName, newOk := s.readObjectName()
			if newOk {
				refs = append(refs, SQLObjectRef{Name: newName, Kind: "table"})
			}
		} else if s.peekWord() == "COLUMN" {
			s.next()
			s.readObjectName()
			if s.consumeWord("TO") {
				s.readObjectName()
			}
		}
	}
	return &SQLStatement{Raw: raw, Type: SQLTypeAlterTable, Objects: refs}, nil
}

func parseDropStatement(s *sqlTokenStream, raw string) (*SQLStatement, error) {
	s.next()
	if s.peekWord() == "IF" {
		s.next()
		s.consumeWord("EXISTS")
	}
	objType := s.peekWord()
	switch objType {
	case "TABLE":
		s.next()
		if s.peekWord() == "IF" {
			s.skipIFClause()
		}
		name, ok := s.readObjectName()
		if !ok {
			return nil, fmt.Errorf("%w: DROP TABLE missing table name", ErrSQLUnparseable)
		}
		return &SQLStatement{Raw: raw, Type: SQLTypeDropTable, Objects: []SQLObjectRef{{Name: name, Kind: "table"}}}, nil
	case "VIEW":
		s.next()
		if s.peekWord() == "IF" {
			s.skipIFClause()
		}
		name, ok := s.readObjectName()
		if !ok {
			return nil, fmt.Errorf("%w: DROP VIEW missing view name", ErrSQLUnparseable)
		}
		return &SQLStatement{Raw: raw, Type: SQLTypeDropView, Objects: []SQLObjectRef{{Name: name, Kind: "view"}}}, nil
	case "TRIGGER":
		s.next()
		if s.peekWord() == "IF" {
			s.skipIFClause()
		}
		name, ok := s.readObjectName()
		if !ok {
			return nil, fmt.Errorf("%w: DROP TRIGGER missing trigger name", ErrSQLUnparseable)
		}
		return &SQLStatement{Raw: raw, Type: SQLTypeDropTrigger, Objects: []SQLObjectRef{{Name: name, Kind: "trigger"}}}, nil
	case "INDEX":
		s.next()
		if s.peekWord() == "IF" {
			s.skipIFClause()
		}
		name, ok := s.readObjectName()
		if !ok {
			return nil, fmt.Errorf("%w: DROP INDEX missing index name", ErrSQLUnparseable)
		}
		return &SQLStatement{Raw: raw, Type: SQLTypeDropIndex, Objects: []SQLObjectRef{{Name: name, Kind: "index"}}}, nil
	default:
		return nil, fmt.Errorf("%w: unsupported DROP %s", ErrSQLUnparseable, objType)
	}
}

func parseInsertStatement(s *sqlTokenStream, raw string) (*SQLStatement, error) {
	s.next()
	if s.peekWord() == "OR" {
		s.next()
		s.next()
	}
	if !s.consumeWord("INTO") {
		return nil, fmt.Errorf("%w: INSERT missing INTO", ErrSQLUnparseable)
	}
	name, ok := s.readObjectName()
	if !ok {
		return nil, fmt.Errorf("%w: INSERT missing table name", ErrSQLUnparseable)
	}
	refs := []SQLObjectRef{{Name: name, Kind: "table"}}
	extra := extractTableRefsFromStream(s)
	refs = append(refs, extra...)
	return &SQLStatement{Raw: raw, Type: SQLTypeInsert, Objects: refs}, nil
}

func parseUpdateStatement(s *sqlTokenStream, raw string) (*SQLStatement, error) {
	s.next()
	if s.peekWord() == "OR" {
		s.next()
		s.next()
	}
	name, ok := s.readObjectName()
	if !ok {
		return nil, fmt.Errorf("%w: UPDATE missing table name", ErrSQLUnparseable)
	}
	refs := []SQLObjectRef{{Name: name, Kind: "table"}}
	extra := extractTableRefsFromStream(s)
	refs = append(refs, extra...)
	return &SQLStatement{Raw: raw, Type: SQLTypeUpdate, Objects: refs}, nil
}

func parseDeleteStatement(s *sqlTokenStream, raw string) (*SQLStatement, error) {
	s.next()
	if !s.consumeWord("FROM") {
		return nil, fmt.Errorf("%w: DELETE missing FROM", ErrSQLUnparseable)
	}
	name, ok := s.readObjectName()
	if !ok {
		return nil, fmt.Errorf("%w: DELETE missing table name", ErrSQLUnparseable)
	}
	refs := []SQLObjectRef{{Name: name, Kind: "table"}}
	extra := extractTableRefsFromStream(s)
	refs = append(refs, extra...)
	return &SQLStatement{Raw: raw, Type: SQLTypeDelete, Objects: refs}, nil
}

func parseSelectStatement(s *sqlTokenStream, raw string) (*SQLStatement, error) {
	refs := extractTableRefsFromStream(s)
	return &SQLStatement{Raw: raw, Type: SQLTypeSelect, Objects: refs}, nil
}

func collectCTENames(s *sqlTokenStream) map[string]bool {
	cteNames := map[string]bool{}
	tokens := s.tokens
	for i := s.pos; i < len(tokens); i++ {
		if tokens[i].kind != tokWord {
			continue
		}
		if strings.ToUpper(tokens[i].value) != "WITH" {
			continue
		}
		j := i + 1
		for j < len(tokens) {
			if tokens[j].kind == tokWord && strings.ToUpper(tokens[j].value) == "AS" {
				if j > 0 && tokens[j-1].kind == tokWord {
					cteNames[strings.ToLower(tokens[j-1].value)] = true
				}
				j++
				if j < len(tokens) && tokens[j].kind == tokPunct && tokens[j].value == "(" {
					depth := 1
					j++
					for j < len(tokens) && depth > 0 {
						if tokens[j].kind == tokPunct && tokens[j].value == "(" {
							depth++
						} else if tokens[j].kind == tokPunct && tokens[j].value == ")" {
							depth--
						}
						j++
					}
				}
				if j < len(tokens) && tokens[j].kind == tokPunct && tokens[j].value == "," {
					j++
					continue
				}
				break
			}
			j++
		}
		break
	}
	return cteNames
}

func extractTableRefsFromStream(s *sqlTokenStream) []SQLObjectRef {
	var refs []SQLObjectRef
	seen := map[string]bool{}
	cteNames := collectCTENames(s)
	addRef := func(name string) {
		if name == "" {
			return
		}
		key := strings.ToLower(name)
		if isReservedWord(key) {
			return
		}
		if cteNames[key] {
			return
		}
		if !seen[key] {
			seen[key] = true
			refs = append(refs, SQLObjectRef{Name: name, Kind: "table"})
		}
	}
	for {
		tok := s.peek()
		if tok.kind == tokEOF {
			break
		}
		if tok.kind == tokPunct {
			s.next()
			continue
		}
		if tok.kind == tokWord {
			w := strings.ToUpper(tok.value)
			if w == "FROM" || w == "JOIN" || w == "INTO" {
				s.next()
				name, ok := s.readObjectName()
				if ok {
					addRef(name)
				}
				continue
			}
			if w == "USING" {
				s.next()
				nextTok := s.peek()
				if nextTok.kind == tokPunct && nextTok.value == "(" {
					s.next()
					continue
				}
				name, ok := s.readObjectName()
				if ok {
					addRef(name)
				}
				continue
			}
			if w == "UPDATE" {
				s.next()
				if s.peekWord() == "OR" {
					s.next()
					s.next()
				}
				name, ok := s.readObjectName()
				if ok {
					addRef(name)
				}
				continue
			}
		}
		s.next()
	}
	return refs
}

func ParseStatements(raw string) ([]*SQLStatement, error) {
	stmts := splitSQLStatements(raw)
	var result []*SQLStatement
	for _, stmt := range stmts {
		parsed, err := ParseStatement(stmt)
		if err != nil {
			return nil, err
		}
		result = append(result, parsed)
	}
	return result, nil
}

func splitSQLStatements(content string) []string {
	var statements []string
	var buf strings.Builder
	runes := []rune(content)
	n := len(runes)
	i := 0
	singleQuote := false
	doubleQuote := false
	bracketQuote := false
	backtickQuote := false
	lineComment := false
	blockComment := false
	beginDepth := 0
	caseDepth := 0
	flush := func() {
		stmt := strings.TrimSpace(buf.String())
		if stmt == "" || isCommentOnlyStmt(stmt) {
			buf.Reset()
			return
		}
		statements = append(statements, stmt)
		buf.Reset()
	}
	for i < n {
		r := runes[i]
		if lineComment {
			buf.WriteRune(r)
			if r == '\n' {
				lineComment = false
			}
			i++
			continue
		}
		if blockComment {
			buf.WriteRune(r)
			if r == '*' && i+1 < n && runes[i+1] == '/' {
				buf.WriteRune('/')
				i += 2
				blockComment = false
				continue
			}
			i++
			continue
		}
		if singleQuote {
			buf.WriteRune(r)
			if r == '\'' {
				if i+1 < n && runes[i+1] == '\'' {
					buf.WriteRune('\'')
					i += 2
					continue
				}
				singleQuote = false
			}
			i++
			continue
		}
		if doubleQuote {
			buf.WriteRune(r)
			if r == '"' {
				if i+1 < n && runes[i+1] == '"' {
					buf.WriteRune('"')
					i += 2
					continue
				}
				doubleQuote = false
			}
			i++
			continue
		}
		if bracketQuote {
			buf.WriteRune(r)
			if r == ']' {
				bracketQuote = false
			}
			i++
			continue
		}
		if backtickQuote {
			buf.WriteRune(r)
			if r == '`' {
				backtickQuote = false
			}
			i++
			continue
		}
		if r == '-' && i+1 < n && runes[i+1] == '-' {
			lineComment = true
			buf.WriteRune('-')
			buf.WriteRune('-')
			i += 2
			continue
		}
		if r == '/' && i+1 < n && runes[i+1] == '*' {
			blockComment = true
			buf.WriteRune('/')
			buf.WriteRune('*')
			i += 2
			continue
		}
		if r == '\'' {
			singleQuote = true
			buf.WriteRune(r)
			i++
			continue
		}
		if r == '"' {
			doubleQuote = true
			buf.WriteRune(r)
			i++
			continue
		}
		if r == '[' {
			bracketQuote = true
			buf.WriteRune(r)
			i++
			continue
		}
		if r == '`' {
			backtickQuote = true
			buf.WriteRune(r)
			i++
			continue
		}
		if r == ';' {
			if beginDepth > 0 {
				buf.WriteRune(';')
				i++
				continue
			}
			flush()
			i++
			continue
		}
		if isSQLIdentChar(r) {
			word, wordLen := readWord(runes, i)
			switch strings.ToUpper(word) {
			case "BEGIN":
				beginDepth++
			case "CASE":
				caseDepth++
			case "END":
				if caseDepth > 0 {
					caseDepth--
				} else if beginDepth > 0 {
					beginDepth--
				}
			}
			buf.WriteString(word)
			i += wordLen
			continue
		}
		buf.WriteRune(r)
		i++
	}
	flush()
	return statements
}

func isCommentOnlyStmt(stmt string) bool {
	trimmed := strings.TrimSpace(stmt)
	if trimmed == "" {
		return true
	}
	if strings.HasPrefix(trimmed, "--") {
		return true
	}
	if strings.HasPrefix(trimmed, "/*") {
		rest := trimmed[2:]
		for {
			idx := strings.Index(rest, "*/")
			if idx < 0 {
				return true
			}
			rest = strings.TrimSpace(rest[idx+2:])
			if rest == "" {
				return true
			}
			if !strings.HasPrefix(rest, "/*") && !strings.HasPrefix(rest, "--") {
				return false
			}
			if strings.HasPrefix(rest, "--") {
				return true
			}
			rest = rest[2:]
		}
	}
	return false
}

func readWord(runes []rune, i int) (string, int) {
	start := i
	for i < len(runes) && isSQLIdentChar(runes[i]) {
		i++
	}
	return string(runes[start:i]), i - start
}
